CC = riscv64-unknown-elf-gcc
LD = riscv64-unknown-elf-ld
CFLAGS = -march=rv32imaf -mabi=ilp32f

TARGET = main.elf
OBJS = main.o

.PHONY: all clean

all: $(TARGET)

$(TARGET): $(OBJS)
	$(LD) -m elf32lriscv -T link.ld -o $(TARGET) $(OBJS)

main.o: main.S
	$(CC) $(CFLAGS) -c -o main.o main.S

clean:
	rm -f $(TARGET) $(OBJS)
