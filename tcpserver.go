// Package tcpserver implements an extremely fast and flexible IPv4 and
// IPv6 capable TCP server with TLS support, graceful shutdown and some
// TCP tuning options like TCP_FASTOPEN, SO_RESUSEPORT and TCP_DEFER_ACCEPT.
//
// Copyright 2019-2020 Moritz Fain
// Moritz Fain <moritz@fain.io>
//
// Source available at github.com/maurice2k/tcpserver,
// licensed under the MIT license (see LICENSE file).

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

	"github.com/maurice2k/ultrapool"
)

// Server struct
type Server struct {
	noCopy               noCopy
	listenAddr           *net.TCPAddr
	listener             *net.TCPListener
	shutdown             bool
	shutdownDeadline     time.Time
	requestHandler       RequestHandlerFunc
	ctx                  *context.Context
	activeConnections    int32
	maxAcceptConnections int64
	acceptedConnections  int64
	tlsConfig            *tls.Config
	tlsEnabled           bool
	listenConfig         *ListenConfig
	connWaitGroup        sync.WaitGroup
	connStructPool       sync.Pool
	loops                int
	wp                   *ultrapool.WorkerPool
	wpNumShards          int
	ballast              [1024 * 1024 * 10]byte
}

// Connection struct embedding net.Conn
type Connection struct {
	net.Conn
	server *Server
	ctx    *context.Context
	ts     time.Time
	_cacheLinePadding [8]byte
}

// Listener config struct
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

// Request handler function type
type RequestHandlerFunc func(conn *Connection)

var defaultListenConfig *ListenConfig = &ListenConfig{
	SocketReusePort: true,
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
// by using server.ListenTLS() or later by using connection.StartTLS()
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
	if s.GetTLSConfig() == nil {
		return fmt.Errorf("No TLS config set!")
	}
	s.tlsEnabled = true
	return s.Listen()
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

// Gracefully shutdown server but wait no longer than d for active connections.
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

	s.wp = ultrapool.NewWorkerPool(func(task ultrapool.Task) {
		s.serveConn(task.(net.Conn))
	})

	s.wp.SetNumShards(s.GetWorkerpoolShards())
	s.wp.Start()
	defer s.wp.Stop()

	loops := s.GetLoops()
	errChan := make(chan error, loops)

	for i := 0; i < loops; i++ {
		go func(id int) {
			errChan <- s.acceptLoop(id)
		}(i)
	}

	for i := 0; i < loops; i++ {
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

// Main accept loop
func (s *Server) acceptLoop(id int) error {
	var (
		tempDelay time.Duration
		conn      net.Conn
		err       error
	)

	for {
		if s.maxAcceptConnections > 0 && s.acceptedConnections >= s.maxAcceptConnections {
			s.Shutdown(0)
		}

		if s.shutdown {
			_ = s.listener.Close()
			break
		}

		conn, err = s.listener.AcceptTCP()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok {

				if opErr.Timeout() {
					tempDelay = 0
					continue
				}

				if !(opErr.Temporary() && opErr.Timeout()) && s.shutdown {
					break
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
			conn.Close()
			continue
		}

		s.wp.AddTask(conn)
		//go s.serveConn(conn)
		conn = nil
	}
	return nil
}

// Serve a single connection
func (s *Server) serveConn(conn net.Conn) {
	atomic.AddInt32(&s.activeConnections, 1)

	if s.tlsEnabled {
		conn = tls.Server(conn, s.GetTLSConfig())
	}

	var myConn *Connection
	v := s.connStructPool.Get()
	if v == nil {
		myConn = &Connection{
			server: s,
			Conn:   conn,
		}
	} else {
		myConn = v.(*Connection)
		myConn.Conn = conn
	}

	myConn.ts = time.Now()

	s.requestHandler(myConn)
	conn.Close()
	myConn.ctx = nil
	myConn.Conn = nil
	s.connStructPool.Put(myConn)

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

// Sets number of accept loops
func (s *Server) SetLoops(loops int) {
	s.loops = loops
}

// Returns number of accept loops (defaults to GOMAXPROCS)
func (s *Server) GetLoops() int {
	if s.loops < 1 {
		s.loops = runtime.GOMAXPROCS(0)
	}
	return s.loops
}

// Sets number of workerpool shards
func (s *Server) SetWorkerpoolShards(num int) {
	s.wpNumShards = num
}

// Returns number of workerpool shards (defaults to GetLoops() * 2)
func (s *Server) GetWorkerpoolShards() int {
	if s.wpNumShards < 1 {
		s.wpNumShards = s.GetLoops() * 2
	}
	return s.wpNumShards
}

// Starts TLS inline
func (conn *Connection) StartTLS(config *tls.Config) error {
	if config == nil {
		config = conn.GetServer().GetTLSConfig()
	}
	if config == nil {
		return fmt.Errorf("no valid TLS config given")
	}
	conn.Conn = tls.Server(conn.Conn, config)
	return nil
}

// Checks whether given net.TCPAddr is a IPv6 address
func IsIPv6Addr(addr *net.TCPAddr) bool {
	return addr.IP.To4() == nil && len(addr.IP) == net.IPv6len
}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
