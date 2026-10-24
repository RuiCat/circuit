* divider: V1 5V, R1 1k, R2 2k
V1 1 0 DC 5
R1 1 2 1k
R2 2 0 2k
.control
tran 1e-7 1e-4 uic
print v(2)
.endc
.end
