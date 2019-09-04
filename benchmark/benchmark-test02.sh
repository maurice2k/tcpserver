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
cpus=16

run_server() {
    GOMAXPROCS=$1
    export GOMAXPROCS
    echo "Starting server: $2"
    eval "$2 &"
    pid=$!
}

kill_server() {
    disown $pid 2>/dev/null && kill $pid 2>/dev/null
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
        run_server $i "./test_http_server $2" #  1>/dev/null 2>/dev/null'
        sleep 2
        server_running=`ps aux |grep "[t]est_http_server" |wc -l`
        if [ $server_running -ne 1 ]
        then
            echo "Server not running! Something weird happend; exiting"
            exit
        fi

        results+=(`$bombardier -c 50 -d 10s $3 --fasthttp |tee /dev/tty |grep -o 'Reqs/sec.*' |awk '{print $2}'`)

        kill_server
    done

    echo "Test finished"
    rm test_http_server 2>/dev/null
}

plot_results() {
    echo "GOMAXPROCS evio tcpserver" >$1-results.dat
    for ((i=0; i<$cpus; i++))
    do
        echo "$(($i+1)) ${results_evio[$i]} ${results_tcpserver[$i]}" >>$1-results.dat
    done

    gnuplot -e "results='$1-results.dat'" plotter.txt
    mv graph.png "$1-graph.png"
}


### TEST #2, 1024 byte, keepalive off

test_http_server 'evio-http-server/main.go' '-keepalive=0 -listen=127.0.0.20:8080 -aaaa=1024 -sleep=0 -loops=`echo $GOMAXPROCS`' 'http://127.0.0.20:8080/'
results_evio=("${results[@]}")
echo ""

test_http_server '../examples/http-server/main.go' '-keepalive=0 -listen=127.0.0.21:8080 -aaaa=1024 -sleep=0' 'http://127.0.0.21:8080/'
results_tcpserver=("${results[@]}")
echo ""

plot_results "test02"
