* rlc: DC 工作点（电感短路、电容开路），v(3)=5
V1 1 0 DC 5
R1 1 2 100
L1 2 3 1m
C1 3 0 1u
.control
tran 1e-7 1e-4 uic
print v(3)
.endc
.end
