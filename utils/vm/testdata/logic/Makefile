CC = riscv64-unknown-elf-gcc
LD = riscv64-unknown-elf-ld
CFLAGS = -march=rv32imafd -mabi=ilp32f -O2 -nostdlib -ffreestanding

TARGET = main.elf
OBJS = main.o

.PHONY: all clean

all: $(TARGET)

$(TARGET): $(OBJS)
	$(LD) -m elf32lriscv -T link.ld -o $(TARGET) $(OBJS)

main.o: main.c
	$(CC) $(CFLAGS) -c -o main.o main.c

clean:
	rm -f $(TARGET) $(OBJS)
