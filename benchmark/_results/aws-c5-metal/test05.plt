set term png

set terminal png size 1200,500
set output 'test05.png'

set grid
set linetype 1 lc rgb '#9400D3'
set linetype 2 lc rgb '#009E73'
set linetype 3 lc rgb '#56B4E9'
set linetype 4 lc rgb '#E69F00'
set linetype 5 lc rgb '#F0E442'
set linetype 6 lc rgb '#0072B2'

set ylabel "requests/sec"
set format y "%'.0f"
set xlabel "GOMAXPROCS"
set style data histogram
set style histogram cluster gap 1
set style fill solid border -1
set boxwidth 0.8
set xtics rotate by -45 scale 0
set key outside right above

stats 'test05.dat' matrix rowheaders columnheaders noout
set autoscale ymax

set ytics 1000

if (STATS_max > 50000) {
    set ytics 5000
}

if (STATS_max > 100000) {
    set ytics 10000
}

if (STATS_max > 500000) {
    set ytics 50000
}

if (STATS_max > 1500000) {
    set ytics 100000
}

plot 'test05.dat' \
    u 'net/http':xticlabels(1) ti col lt 1, ''\
    u 'evio':xticlabels(1) ti col lt 2, ''\
    u 'gnet':xticlabels(1) ti col lt 3, ''\
    u 'fasthttp':xticlabels(1) ti col lt 4, ''\
    u 'tcpserver':xticlabels(1) ti col lt 5
