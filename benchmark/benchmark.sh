#!/bin/bash

bombardier='/opt/bombardier-linux-amd64'
jq='/usr/bin/jq'

numactl_http_server='numactl -N0'
numactl_bombardier='numactl -N1'

cpus=`grep ^processor /proc/cpuinfo |wc -l`
conns=1000
duration=10   # duration of test in seconds

if [[ -n $CPUS && $CPUS -gt 0 ]]
then
    cpus=$CPUS
fi

if [[ -n $CONNS && $CONNS -gt 0 ]]
then
    conns=$CONNS
fi

if [[ -n $DURATION && $DURATION -gt 0 ]]
then
    duration=$DURATION
fi

if [[ -n $DURATION && $DURATION -gt 0 ]]
then
    duration=$DURATION
fi


## kill every process that matches "test_http_server"
ps a |grep "[t]est_http_serv" |awk '{print $1}' |xargs -I{} kill -9 {}

run_server() {
    echo "Starting server: $2"
    eval "GOMAXPROCS=$1 $numactl_http_server $2 &"
    pid=$!
}

kill_server() {
    disown $pid 2>/dev/null ; kill -9 $pid 2>/dev/null
}

declare -a results

test_http_server() {
    results=()
    used_cpus=()
    rm test_http_server 2>/dev/null
    killall -9 test_http_server 2>/dev/null

    echo "Building $1"
    go build -mod vendor -o test_http_server $1

    start_cpu=1
    exact=0
    if [[ -n $START_CPU && $START_CPU -gt 0 ]]
    then
        start_cpu=$START_CPU
    fi

    if [[ -n $ONLYGIVENCPUS && $ONLYGIVENCPUS -eq 1 ]]
    then
        echo "Testing with exactly $cpus CPU(s)"
        start_cpu=$cpus
        exact=1
    else
        echo "Testing with $start_cpu..$cpus CPUs"
    fi

    for ((i=$start_cpu; i<=$cpus; i++))
    do
        if [[ $exact -eq 0 && $i -ne $cpus ]]; then
            if [[ $cpus -gt 12 && $i -gt 4 && $(($i%2)) -ne 0 ]]; then
                continue
            fi
            if [[ $i -gt 16 && $(($i%3)) -ne 0 ]]; then
                continue
            fi
        fi
        port=$((8000+$i))
        params=${2/"{port}"/$port}
        for ((j=0; j<3; j++))  ## max. 3 retries
        do
            run_server $i "./test_http_server $params" #  1>/dev/null 2>/dev/null'
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

        url=${3/"{port}"/$port}
        used_cpus+=($i)
        #result=`GOGC=400 $numactl_bombardier $bombardier -c $conns -d ${duration}s -pr -oj --fasthttp $url |$jq . |tee /dev/fd/2 |$jq '.|select(.result.others==0) .result.rps.mean//0'`
        result=`GOGC=400 $numactl_bombardier $bombardier -c $conns -d ${duration}s -pr -oj --fasthttp $url |$jq '.|select(.result.others==0) .result.rps.mean//0'`
        echo "--> $result reqs/sec @GOMAXPROCS=$i" >/dev/fd/2
        results+=($result)

        kill_server
        sleep 3
    done

    rm test_http_server 2>/dev/null
}

plot_results() {
    echo -e "GOMAXPROCS\tnet/http\tevio\tgnet\tfasthttp\ttcpserver" >$1-results.dat

    for ((i=0; i<${#used_cpus[@]}; i++))
    do
        echo -e "${used_cpus[$i]}\t${results_net[$i]:-0}\t${results_evio[$i]:-0}\t${results_gnet[$i]:-0}\t${results_fasthttp[$i]:-0}\t${results_tcpserver[$i]:-0}" >>$1-results.dat
    done

    gnuplot -e "testcase='$1'" plotter.txt
}


run_test1() {
    echo "====[ Running test #1: HTTP returning 1024 byte, ${conns} concurrent connections, keepalive off ]===="

    test_http_server 'net-http-server/main.go' '-keepalive=0 -listen=127.0.0.10:{port} -aaaa=1024 -sleep=0' 'http://127.0.0.10:{port}/'
    results_net=("${results[@]}")
    echo ""

    test_http_server 'evio-http-server/main.go' '-keepalive=0 -listen=127.0.0.11:{port} -aaaa=1024 -sleep=0 -loops=-1' 'http://127.0.0.11:{port}/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'gnet-http-server/main.go' '-keepalive=0 -listen=127.0.0.12:{port} -aaaa=1024 -sleep=0 -loops=-1' 'http://127.0.0.12:{port}/'
    results_gnet=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=0 -listen=127.0.0.13:{port} -aaaa=1024 -sleep=0' 'http://127.0.0.13:{port}/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=0 -listen=127.0.0.14:{port} -aaaa=1024 -sleep=0' 'http://127.0.0.14:{port}/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test01"
    echo "FINISHED."
    echo ""
}


run_test2() {
    echo "====[ Running test #2: HTTP returning 1024 byte, ${conns} concurrent connections, keepalive on ]===="

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.20:{port} -aaaa=1024 -sleep=0' 'http://127.0.0.20:{port}/'
    results_net=("${results[@]}")
    echo ""

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.21:{port} -aaaa=1024 -sleep=0 -loops=-1' 'http://127.0.0.21:{port}/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'gnet-http-server/main.go' '-keepalive=1 -listen=127.0.0.22:{port} -aaaa=1024 -sleep=0 -loops=-1' 'http://127.0.0.22:{port}/'
    results_gnet=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.23:{port} -aaaa=1024 -sleep=0' 'http://127.0.0.23:{port}/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.24:{port} -aaaa=1024 -sleep=0' 'http://127.0.0.24:{port}/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test02"
    echo "FINISHED."
    echo ""
}


run_test3() {
    echo "====[ Running test #3: HTTP returning AES128(1024 byte), ${conns} concurrent connections, keepalive off ]===="

    test_http_server 'net-http-server/main.go' '-keepalive=0 -listen=127.0.0.30:{port} -aaaa=1024 -aes128 -sleep=0' 'http://127.0.0.30:{port}/'
    results_net=("${results[@]}")
    echo ""

    test_http_server 'evio-http-server/main.go' '-keepalive=0 -listen=127.0.0.31:{port} -aaaa=1024 -aes128 -sleep=0 -loops=-1' 'http://127.0.0.31:{port}/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'gnet-http-server/main.go' '-keepalive=0 -listen=127.0.0.32:{port} -aaaa=1024 -aes128 -sleep=0 -loops=-1' 'http://127.0.0.32:{port}/'
    results_gnet=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=0 -listen=127.0.0.33:{port} -aaaa=1024 -aes128 -sleep=0' 'http://127.0.0.33:{port}/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=0 -listen=127.0.0.34:{port} -aaaa=1024 -aes128 -sleep=0' 'http://127.0.0.34:{port}/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test03"
    echo "FINISHED."
    echo ""
}


run_test4() {
    echo "====[ Running test #4: HTTP returning AES128(1024 byte), ${conns} concurrent connections, keepalive on ]===="

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.40:{port} -aaaa=1024 -aes128 -sleep=0' 'http://127.0.0.40:{port}/'
    results_net=("${results[@]}")
    echo ""

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.41:{port} -aaaa=1024 -aes128 -sleep=0 -loops=-1' 'http://127.0.0.41:{port}/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'gnet-http-server/main.go' '-keepalive=1 -listen=127.0.0.42:{port} -aaaa=1024 -aes128 -sleep=0 -loops=-1' 'http://127.0.0.42:{port}/'
    results_gnet=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.43:{port} -aaaa=1024 -aes128 -sleep=0' 'http://127.0.0.43:{port}/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.44:{port} -aaaa=1024 -aes128 -sleep=0' 'http://127.0.0.44:{port}/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test04"
    echo "FINISHED."
    echo ""
}


run_test5() {
    echo "====[ Running test #5: HTTP returning 128 byte, ${conns} concurrent connections, keepalive on, sleep 1 ms ]===="

    test_http_server 'net-http-server/main.go' '-keepalive=1 -listen=127.0.0.50:{port} -aaaa=128 -sleep=1' 'http://127.0.0.50:{port}/'
    results_net=("${results[@]}")
    echo ""

    test_http_server 'evio-http-server/main.go' '-keepalive=1 -listen=127.0.0.51:{port} -aaaa=128 -sleep=1 -loops=-1' 'http://127.0.0.51:{port}/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'gnet-http-server/main.go' '-keepalive=1 -listen=127.0.0.52:{port} -aaaa=128 -sleep=1 -loops=-1' 'http://127.0.0.52:{port}/'
    results_gnet=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=1 -listen=127.0.0.53:{port} -aaaa=128 -sleep=1' 'http://127.0.0.53:{port}/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=1 -listen=127.0.0.54:{port} -aaaa=128 -sleep=1' 'http://127.0.0.54:{port}/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test05"
    echo "FINISHED."
    echo ""
}


run_test6() {
    echo "====[ Running test #6: HTTP returning 16384 byte, ${conns} concurrent connections, keepalive off ]===="

    test_http_server 'net-http-server/main.go' '-keepalive=0 -listen=127.0.0.60:{port} -aaaa=16384 -sleep=0' 'http://127.0.0.60:{port}/'
    results_net=("${results[@]}")
    echo ""

    test_http_server 'evio-http-server/main.go' '-keepalive=0 -listen=127.0.0.61:{port} -aaaa=16384 -sleep=0 -loops=-1' 'http://127.0.0.61:{port}/'
    results_evio=("${results[@]}")
    echo ""

    test_http_server 'gnet-http-server/main.go' '-keepalive=0 -listen=127.0.0.62:{port} -aaaa=16384 -sleep=0 -loops=-1' 'http://127.0.0.62:{port}/'
    results_gnet=("${results[@]}")
    echo ""

    test_http_server 'fasthttp-http-server/main.go' '-keepalive=0 -listen=127.0.0.63:{port} -aaaa=16384 -sleep=0' 'http://127.0.0.63:{port}/'
    results_fasthttp=("${results[@]}")
    echo ""

    test_http_server '../examples/http-server/main.go' '-keepalive=0 -listen=127.0.0.64:{port} -aaaa=16384 -sleep=0' 'http://127.0.0.64:{port}/'
    results_tcpserver=("${results[@]}")
    echo ""

    plot_results "test06"
    echo "FINISHED."
    echo ""
}

run_install() {
    pwd=`pwd`
    sudo apt install gnuplot git jq mc screen pv numactl -y
    cd /opt
    sudo wget https://dl.google.com/go/go1.16rc1.linux-amd64.tar.gz ; sudo tar xzf go1.16rc1.*.tar.gz
    echo "export PATH=/opt/go/bin:$PATH" > /etc/profile.d/go
    echo "export GOROOT=/opt/go" >> /etc/profile.d/go
    source /etc/profile.d/go
    sudo wget https://github.com/codesenberg/bombardier/releases/download/v1.2.5/bombardier-linux-amd64 ; sudo chmod +x /opt/bombardier-linux-amd64
    cd $pwd
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
test1)    run_test1
          ;;
test2)    run_test2
          ;;
test3)    run_test3
          ;;
test4)    run_test4
          ;;
test5)    run_test5
          ;;
test6)    run_test6
          ;;
install)  run_install
          ;;
*)        run_all_tests
          ;;
esac
exit 0
