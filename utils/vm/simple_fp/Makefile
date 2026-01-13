TARGET = main
GO_TEST_TARGET = simple_fp_test

CROSS_COMPILE = riscv64-unknown-elf-
CC = $(CROSS_COMPILE)gcc
AS = $(CROSS_COMPILE)as
LD = $(CROSS_COMPILE)ld
OBJDUMP = $(CROSS_COMPILE)objdump

CFLAGS = -nostdlib -nostartfiles -march=rv32imafd -mabi=ilp32f -Tlink.ld

.PHONY: all clean test

all: $(TARGET).elf

$(TARGET).o: $(TARGET).S
	$(CC) $(CFLAGS) -c $< -o $@

$(TARGET).elf: $(TARGET).o
	$(LD) -m elf32lriscv -Tlink.ld $< -o $@

clean:
	rm -f *.o *.elf

test:
	go test -v -run ^TestRunSimpleFPELF$
