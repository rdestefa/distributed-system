#!/bin/python

import sys
import numpy as np

values = [float(line) for line in sys.stdin]
count = len(values)
total = sum(values)
average = total/count
maximum = max(values)
minimum = min(values)
stddev = np.std(values)

print(count, total, average, maximum, minimum, stddev)
