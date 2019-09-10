// Copyright 2019 Moritz Fain
// Moritz Fain <moritz@fain.io>
package tcpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gammazero/workerpool"
)

type Server struct {
	noCopy noCopy

	listenAddr           *net.TCPAddr
	listener             advancedListener
	shutdown             bool
	shutdownDeadline     time.Time
	requestHandler       RequestHandlerFunc
	ctx                  *context.Context
	activeConnections    int32
	maxAcceptConnections int64
	acceptedConnections  int64
	tlsConfig            *tls.Config
	listenConfig         *ListenConfig
	connWaitGroup        sync.WaitGroup

	wp *workerpool.WorkerPool
}

type Connection struct {
	net.Conn
	server *Server
	ctx    *context.Context
	ts     time.Time
}

type ListenConfig struct {
	lc net.ListenConfig
	// Enable/disable SO_REUSEPORT (requires Linux >=2.4)
	SocketReusePort bool
	// Enable/disable TCP_FASTOPEN (requires Linux >=3.7 or Windows 10, version 1607)
	// For Linux:
	// - see https://lwn.net/Articles/508865/
	// - enable with "echo 3 >/proc/sys/net/ipv4/tcp_fastopen" for client and server
	// For Windows:
	// - enable with "netsh int tcp set global fastopen=enabled"
	SocketFastOpen bool
	// Queue length for TCP_FASTOPEN (default 256)
	SocketFastOpenQueueLen int
	// Enable/disable TCP_DEFER_ACCEPT (requires Linux >=2.4)
	SocketDeferAccept bool
}

var defaultListenConfig *ListenConfig = &ListenConfig{
	SocketReusePort: true,
}

type RequestHandlerFunc func(conn *Connection)

// net.TCPListener satisfies advancedListener interface
type advancedListener interface {
	net.Listener
	SetDeadline(t time.Time) error
}

// Creates a new server instance
func NewServer(listenAddr string) (*Server, error) {
	la, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("error resolving address '%s': %s", listenAddr, err)
	}
	return &Server{
		listenAddr:   la,
		listenConfig: defaultListenConfig,
	}, nil
}

// Sets TLS config but does not enable TLS yet. TLS can be either enabled
// by using server.ListebTLS() or later by using connection.StartTLS()
func (s *Server) SetTLSConfig(config *tls.Config) {
	s.tlsConfig = config
}

// Returns previously set TLS config
func (s *Server) GetTLSConfig() *tls.Config {
	return s.tlsConfig
}

// Sets listen config
func (s *Server) SetListenConfig(config *ListenConfig) {
	s.listenConfig = config
}

// Returns listen config
func (s *Server) GetListenConfig() *ListenConfig {
	return s.listenConfig
}

// Starts listening
func (s *Server) Listen() (err error) {
	network := "tcp4"
	if IsIPv6Addr(s.listenAddr) {
		network = "tcp6"
	}

	s.listenConfig.lc.Control = applyListenSocketOptions(s.listenConfig)
	l, err := s.listenConfig.lc.Listen(*s.GetContext(), network, s.listenAddr.String())
	if err != nil {
		return err
	}
	if tcpl, ok := l.(*net.TCPListener); ok {
		s.listener = tcpl
	} else {
		return fmt.Errorf("listener must be of type net.TCPListener")
	}

	return nil
}

// Starts listening using TLS
func (s *Server) ListenTLS() (err error) {
	err = s.Listen()
	if err != nil {
		return err
	}

	// s.listener is always a TCPListener, so we are safe to assume that the
	// tls.NewListener returned net.Listener{tls.listener} also implements
	// advancedListener as tls.listener only wraps the given listener
	s.listener = tls.NewListener(s.listener, s.GetTLSConfig()).(advancedListener)

	return nil
}

// Sets maximum number of connections that are being accepted before the
// server automatically shutdowns
func (s *Server) SetMaxAcceptConnections(limit int64) {
	atomic.StoreInt64(&s.maxAcceptConnections, limit)
}

// Returns number of currently active connections
func (s *Server) GetActiveConnections() int32 {
	return s.activeConnections
}

// Returns number of accepted connections
func (s *Server) GetAcceptedConnections() int64 {
	return s.acceptedConnections
}

// Returns listening address
func (s *Server) GetListenAddr() *net.TCPAddr {
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr().(*net.TCPAddr)
}

// Gracefully shutdown server but why no longer than d for active connections.
// Use d = 0 to wait indefinitely for active connections.
func (s *Server) Shutdown(d time.Duration) (err error) {
	s.shutdownDeadline = time.Time{}
	if d > 0 {
		s.shutdownDeadline = time.Now().Add(d)
	}
	s.shutdown = true
	err = s.listener.Close()
	if err != nil {
		return err
	}
	return nil
}

// Shutdown server immediately, do not wait for any connections
func (s *Server) Halt() (err error) {
	return s.Shutdown(-1 * time.Second)
}

// Serves requests (accept / handle loop)
func (s *Server) Serve() error {
	if s.listener == nil {
		return fmt.Errorf("no valid listener found; call Listen() or ListenTLS() first")
	}
	goMaxProcs := runtime.GOMAXPROCS(0)
	if goMaxProcs < 1 {
		goMaxProcs = 1
	}

	s.wp = workerpool.New(10000)

	errChan := make(chan error, goMaxProcs)

	for i := 0; i < goMaxProcs; i++ {
		go func() {
			errChan <- s.acceptLoop()
		}()
	}

	for i := 0; i < goMaxProcs; i++ {
		err := <-errChan
		if err != nil {
			return err
		}
	}

	if s.activeConnections == 0 {
		return nil
	}

	if s.shutdownDeadline.IsZero() {
		// just wait for all connections to be closed
		s.connWaitGroup.Wait()

	} else {
		diff := s.shutdownDeadline.Sub(time.Now())
		if diff > 0 {
			// wait specified time for still active connections to be closed
			time.Sleep(diff)
		}
	}

	return nil
}

func (s *Server) acceptLoop() error {
	var tempDelay time.Duration

	for {
		if s.maxAcceptConnections > 0 && s.acceptedConnections >= s.maxAcceptConnections {
			s.Shutdown(0)
		}

		if s.shutdown {
			_ = s.listener.Close()
			break
		}

		_ = s.listener.SetDeadline(time.Now().Add(1 * time.Second))
		conn, err := s.listener.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok {

				if !(opErr.Temporary() && opErr.Timeout()) && s.shutdown {
					break
				}

				if opErr.Timeout() {
					tempDelay = 0
					continue
				}

				if opErr.Temporary() {
					if tempDelay == 0 {
						tempDelay = 10 * time.Millisecond
					} else {
						tempDelay *= 2
					}

					if max := time.Second; tempDelay > max {
						tempDelay = max
					}

					time.Sleep(tempDelay)
					continue
				}

			}

			s.listener.Close()
			return err
		}

		tempDelay = 0

		newAcceptedConns := atomic.AddInt64(&s.acceptedConnections, 1)
		if s.maxAcceptConnections > 0 && newAcceptedConns > s.maxAcceptConnections {
			// We have accepted too much connections which might happen due to
			// the fact that we use multiple accept loops without locking.
			// In this case we just close the connection (we shouldn't have accepted
			// in the first place) and continue for shutting down the server.
			continue
		}

		s.connWaitGroup.Add(1)
		atomic.AddInt32(&s.activeConnections, 1)
		//go s.serveConn(conn)
		s.wp.Submit(func() { s.serveConn(conn) })
	}
	return nil
}

// Serve a single connection
func (s *Server) serveConn(conn net.Conn) {
	myConn := &Connection{
		Conn:   conn,
		server: s,
		ts:     time.Now(),
	}
	s.requestHandler(myConn)
	myConn.Close()
	s.connWaitGroup.Done()
	atomic.AddInt32(&s.activeConnections, -1)
}

// Sets request handler function
func (s *Server) SetRequestHandler(f RequestHandlerFunc) {
	s.requestHandler = f
}

// Sets context to the server that is later passed to the handleRequest method
func (s *Server) SetContext(ctx *context.Context) {
	s.ctx = ctx
}

// Returns server's context or creates a new one if none is present
func (s *Server) GetContext() *context.Context {
	if s.ctx == nil {
		ctx := context.Background()
		s.ctx = &ctx
	}
	return s.ctx
}

// Returns server
func (conn *Connection) GetServer() *Server {
	return conn.server
}

// Returns client IP and port
func (conn *Connection) GetClientAddr() *net.TCPAddr {
	return conn.RemoteAddr().(*net.TCPAddr)
}

// Returns server IP and port (the addr the connection was accepted at)
func (conn *Connection) GetServerAddr() *net.TCPAddr {
	return conn.LocalAddr().(*net.TCPAddr)
}

// Returns start timestamp
func (conn *Connection) GetStartTime() time.Time {
	return conn.ts
}

// Sets context to the connection that is later passed to the handleRequest method
func (conn *Connection) SetContext(ctx *context.Context) {
	conn.ctx = ctx
}

// Returns connection's context or creates a new one if none is present
func (conn *Connection) GetContext() *context.Context {
	if conn.ctx == nil {
		ctx := context.Background()
		conn.ctx = &ctx
	}
	return conn.ctx
}

// Starts TLS inline
func (conn *Connection) StartTLS(config *tls.Config) error {
	if config == nil {
		config = conn.GetServer().GetTLSConfig()
	}
	if config == nil {
		return fmt.Errorf("no valid TLS config given")
	}
	conn.Conn = tls.Server(conn, config)
	return nil
}

func IsIPv6Addr(addr *net.TCPAddr) bool {
	return addr.IP.To4() == nil && len(addr.IP) == net.IPv6len
}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
