* rc filter: DC 工作点（电容开路），v(2)=5
V1 1 0 DC 5
R1 1 2 1k
C1 2 0 1u
.control
tran 1e-7 1e-4 uic
print v(2)
.endc
.end
