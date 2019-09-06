#!/bin/bash

# apt install gnuplot git mc screen pv
# cd /opt
# wget https://github.com/codesenberg/bombardier/releases/download/v1.2.4/bombardier-linux-amd64 ; chmod +x /opt/bombardier-linux-amd64
# wget https://dl.google.com/go/go1.12.9.linux-amd64.tar.gz ; tar xzf go1.12.*tar.gz
# echo "export PATH=/opt/go/bin:$PATH" >> ~/.profile
# echo "export GOROOT=/opt/go" >> ~/.profile
# source ~/.profile
# cd ~/
# git clone git@github.com:maurice2k/tcpserver.git
# cd tcpserver/benchmark
# bash benchmark.sh


bombardier='/opt/bombardier-linux-amd64'

cpus=`grep ^processor /proc/cpuinfo |wc -l`
conns=50
duration=2   # 2 seconds

if [ $CPUS -gt 0 ]
then
    cpus=$CPUS
fi

if [ $CONNS -gt 0 ]
then
    conns=$CONNS
fi

if [ $DURATION -gt 0 ]
then
    duration=$DURATION
fi


run_server() {
    GOMAXPROCS=$1
    export GOMAXPROCS
    echo "Starting server: $2"
    eval "$2 &"
    pid=$!
}

kill_server() {
    disown $pid 2>/dev/null ; kill -9 $pid 2>/dev/null
}

declare -a results

test_http_server() {
    results=()
    rm test_http_server 2>/dev/null
    killall -9 test_http_server 2>/dev/null

    echo "Building $1"
    go build -o test_http_server $1

    echo "Testing with up to $cpus CPUs"
    for ((i=1; i<=$cpus; i++))
    do

        for ((j=0; j<3; j++))
        do
            run_server $i "./test_http_server $2" #  1>/dev/null 2>/dev/null'
            sleep 2
            server_running=`ps aux |grep "[t]est_http_server" |wc -l`
            if [ $server_running -ne 1 ]
            then
                if [ $j -eq 2 ]
                then
                    echo "Server not running! Something weird happend; exiting"
                    exit
                fi
                sleep 2
                continue
            fi
            break
        done

        results+=(`$bombardier -c $conns -d ${duration}s $3 --fasthttp |grep -o 'Reqs/sec.*' |awk '{print $2}'`)

        kill_server
    done

    rm test_http_server 2>/dev/null
}

plot_results() {
    echo "GOMAXPROCS evio fasthttp net/http tcpserver" >$1-results.dat
    for ((i=0; i<$cpus; i++))
    do
        echo "$(($i+1)) ${results_evio[$i]} ${results_fasthttp[$i]} ${results_net[$i]} ${results_tcpserver[$i]}" >>$1-results.dat
    done

    gnuplot -e "results='$1-results.dat'" plotter.txt
    mv graph.png "$1-graph.png"
}


run_test1() {
    echo "====[ Running test #1: HTTP returning 1024 byte, ${conns} concurrent connections, keepalive on ]===="

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.10:8080 -aaaa=1024 -sleep=0 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.10:8080/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.11:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.11:8080/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.12:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.12:8080/'
    results_net=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.13:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.13:8080/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test01"
    echo "FINISHED."
    echo ""
}


run_test2() {
    echo "====[ Running test #2: HTTP returning 1024 byte, ${conns} concurrent connections, keepalive off ]===="

    test_http_server 'evio-http-server/main.go' '-keepalive=0 -listen=127.0.0.20:8080 -aaaa=1024 -sleep=0 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.20:8080/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=0 -listen=127.0.0.21:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.21:8080/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server 'net-http-server/main.go' '-keepalive=0 -listen=127.0.0.22:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.22:8080/'
    results_net=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=0 -listen=127.0.0.23:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.23:8080/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test02"
    echo "FINISHED."
    echo ""
}


run_test3() {
    echo "====[ Running test #3: HTTP returning SHA256(1024 byte), ${conns} concurrent connections, keepalive on ]===="

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.30:8080 -aaaa=1024 -sha -sleep=0 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.30:8080/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.31:8080 -aaaa=1024 -sha -sleep=0' 'http://127.0.0.31:8080/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.32:8080 -aaaa=1024 -sha -sleep=0' 'http://127.0.0.32:8080/'
    results_net=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.33:8080 -aaaa=1024 -sha -sleep=0' 'http://127.0.0.33:8080/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test03"
    echo "FINISHED."
    echo ""
}


run_test4() {
    echo "====[ Running test #4: HTTP returning SHA256(1024 byte), ${conns} concurrent connections, keepalive off ]===="

    test_http_server 'evio-http-server/main.go' '-keepalive=0 -listen=127.0.0.40:8080 -aaaa=1024 -sha -sleep=0 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.40:8080/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=0 -listen=127.0.0.41:8080 -aaaa=1024 -sha -sleep=0' 'http://127.0.0.41:8080/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server 'net-http-server/main.go' '-keepalive=0 -listen=127.0.0.42:8080 -aaaa=1024 -sha -sleep=0' 'http://127.0.0.42:8080/'
    results_net=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=0 -listen=127.0.0.43:8080 -aaaa=1024 -sha -sleep=0' 'http://127.0.0.43:8080/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test04"
    echo "FINISHED."
    echo ""
}


run_test5() {
    echo "====[ Running test #5: HTTP returning 128 byte, ${conns} concurrent connections, keepalive on, sleep 1 ms ]===="

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.50:8080 -aaaa=128 -sleep=1 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.50:8080/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.51:8080 -aaaa=128 -sleep=1' 'http://127.0.0.51:8080/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.52:8080 -aaaa=128 -sleep=1' 'http://127.0.0.52:8080/'
    results_net=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.53:8080 -aaaa=128 -sleep=1' 'http://127.0.0.53:8080/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test05"
    echo "FINISHED."
    echo ""
}


run_test6() {
    echo "====[ Running test #6: HTTP returning 4096 byte, ${conns} concurrent connections, keepalive on ]===="

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.60:8080 -aaaa=4096 -sleep=0 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.60:8080/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.61:8080 -aaaa=4096 -sleep=0' 'http://127.0.0.61:8080/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.62:8080 -aaaa=4096 -sleep=0' 'http://127.0.0.62:8080/'
    results_net=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.63:8080 -aaaa=4096 -sleep=0' 'http://127.0.0.63:8080/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test06"
    echo "FINISHED."
    echo ""
}

run_all_tests() {
    run_test1
    run_test2
    run_test3
    run_test4
    run_test5
    run_test6
}

case "$1" in
test1)  run_test1
        ;;
test2)  run_test2
        ;;
test3)  run_test3
        ;;
test4)  run_test4
        ;;
test5)  run_test5
        ;;
test6)  run_test6
        ;;
*)      run_all_tests
        ;;
esac
exit 0
