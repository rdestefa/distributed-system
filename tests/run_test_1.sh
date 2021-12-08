#!/bin/bash

for N in 10 20 30 40 50 60 70 80 90 100; do
    echo N = $N
    for n in $(seq 5); do
        rm out-test\ *
        echo $n
        python test.py $N > /dev/null
        cat out-test\ * | ./test_1_stats.py >> test_1_results/N$N.txt
        cat test_1_results/N$N.txt;
    done
done

rm out-test\ *
