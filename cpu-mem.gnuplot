set ytics  nomirror
set y2tics nomirror

set xlabel 'Time (ms)'
set ylabel 'CPU (s/s %)'
set y2label 'Memory alloc (MB)'

set style line 1 lc rgb '#8b1a0e' pt 1 ps 1 lt 1 lw 2
set style line 2 lc rgb '#5e9c36' pt 1 ps 1 lt 1 lw 2

set terminal svg size 800,400 fname 'Verdana'
set output output

plot input using 2:7 with lines linetype 1 axis x1y1 title 'CPU%', \
     input using 2:8 with lines linetype 2 axis x1y2 title 'mem (MB)'

