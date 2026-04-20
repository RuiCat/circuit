/*****************************************************************************
**                      Copyright  (C)  WCH  2001-2022                      **
**                      Web:  http://wch.cn                                 **
******************************************************************************
Abstract:
    USB2.0转接芯片CH347,基于480Mbps高速USB总线扩展UART、SPI、IIC、JTAG
Environment:
Linux
Notes:
Copyright (c) 2022 Nanjing Qinheng Microelectronics Co., Ltd.
--*/

#ifndef _CH347LIB_H
#define _CH347LIB_H

#ifndef CHAR
#define CHAR char
#endif

#ifndef UCHAR
#define UCHAR unsigned char
#endif

#ifndef USHORT
#define USHORT unsigned short
#endif

#ifndef ULONG
#define ULONG unsigned long
#endif

#ifndef LONGLONG
#define LONGLONG unsigned long long
#endif

#ifndef PUCHAR
#define PUCHAR unsigned char *
#endif

#ifndef PCHAR
#define PCHAR char *
#endif

#ifndef PUSHORT
#define PUSHORT unsigned short *
#endif

#ifndef PULONG
#define PULONG unsigned long *
#endif

#ifndef VOID
#define VOID void
#endif

#ifndef PVOID
#define PVOID void *
#endif

typedef enum
{
    FALSE = 0,
    TRUE,
} BOOL;

#define MAX_PATH 260

//驱动接口
#define CH347_USB_CH341 0
#define CH347_USB_HID 2
#define CH347_USB_VCP 3

//芯片功能接口号
#define CH347_FUNC_UART 0
#define CH347_FUNC_SPI_IIC 1
#define CH347_FUNC_JTAG_IIC 2

#define DEFAULT_READ_TIMEOUT 500  //默认读超时毫秒数
#define DEFAULT_WRITE_TIMEOUT 500 //默认写超时毫秒数

#pragma pack(1)
// SPI控制器配置
typedef struct _SPI_CONFIG
{
    UCHAR iMode;                  // 0-3:SPI Mode0/1/2/3
    UCHAR iClock;                 // 0=60MHz, 1=30MHz, 2=15MHz, 3=7.5MHz, 4=3.75MHz, 5=1.875MHz, 6=937.5KHz，7=468.75KHz
    UCHAR iByteOrder;             // 0=低位在前(LSB), 1=高位在前(MSB)
    USHORT iSpiWriteReadInterval; // SPI接口常规读取写入数据命令，单位为uS
    UCHAR iSpiOutDefaultData;     // SPI读数据时默认输出数据
    ULONG iChipSelect;            // 片选控制, 位7为0则忽略片选控制, 位7为1则参数有效: 位1位0为00/01分别选择CS1/CS2引脚作为低电平有效片选
    UCHAR CS1Polarity;            // 位0：片选CS1极性控制：0：低电平有效；1：有电平有效；
    UCHAR CS2Polarity;            // 位0：片选CS1极性控制：0：低电平有效；1：有电平有效；
    USHORT iIsAutoDeativeCS;      // 操作完成后是否自动撤消片选
    USHORT iActiveDelay;          // 设置片选后执行读写操作的延时时间,单位us
    ULONG iDelayDeactive;         // 撤消片选后执行读写操作的延时时间,单位us
} mSpiCfgS, *mPSpiCfgS;

//设备信息
typedef struct _DEV_INFOR
{
    UCHAR iIndex;               // 当前打开序号
    UCHAR DevicePath[MAX_PATH]; // 设备链接名,用于CreateFile
    UCHAR UsbClass;             // 0:CH347_USB_CH341, 2:CH347_USB_HID,3:CH347_USB_VCP
    UCHAR FuncType;             // 0:CH347_FUNC_UART,1:CH347_FUNC_SPI_IIC,2:CH347_FUNC_JTAG_IIC
    CHAR DeviceID[64];          // USB\VID_xxxx&PID_xxxx
    UCHAR ChipMode;             // 芯片模式,0:Mode0(UART0/1); 1:Mode1(Uart1+SPI+IIC); 2:Mode2(HID Uart1+SPI+IIC) 3:Mode3(Uart1+Jtag+IIC)
                                // HANDLE   DevHandle;              // 设备句柄
    int DevHandle;
    USHORT BulkOutEndpMaxSize;   // 上传端点大小
    USHORT BulkInEndpMaxSize;    // 下传端点大小
    UCHAR UsbSpeedType;          // USB速度类型，0:FS,1:HS,2:SS
    UCHAR CH347IfNum;            // 设备接口号: 0:UART,1:SPI/IIC/JTAG/GPIO
    UCHAR DataUpEndp;            // 端点地址
    UCHAR DataDnEndp;            // 端点地址
    CHAR ProductString[64];      // USB产品字符串
    CHAR ManufacturerString[64]; // USB厂商字符串
    ULONG WriteTimeout;          // USB写超时
    ULONG ReadTimeout;           // USB读超时
    CHAR FuncDescStr[64];        // 接口功能描述符
    UCHAR FirewareVer;           // 固件版本
    ULONG CmdDataMaxSize;
} mDeviceInforS, *mPDeviceInforS;
#pragma pack()

#define USBCLASS 3      // 使用接口选择 2：HID、3：厂商驱动

// CH347模式公用函数,支持CH347所有模式下的打开、关闭、USB读、USB写，包含HID
//打开USB设备
int CH347OpenDevice(ULONG iIndex);

//关闭USB设备
BOOL CH347CloseDevice(ULONG iIndex);

//获取设备信息
BOOL CH347GetDeviceInfor(ULONG iIndex, mDeviceInforS *DevInformation);

typedef VOID (*mPCH347_NOTIFY_ROUTINE)( // 设备事件通知回调程序
    ULONG iEventStatus);                        // 设备事件和当前状态(在下行定义): 0=设备拔出事件, 3=设备插入事件

#define CH347_DEVICE_ARRIVAL 3     // 设备插入事件,已经插入
#define CH347_DEVICE_REMOVE_PEND 1 // 设备将要拔出
#define CH347_DEVICE_REMOVE 0      // 设备拔出事件,已经拔出

BOOL CH347SetDeviceNotify(                  // 设定设备事件通知程序
    ULONG iIndex,                           // 指定设备序号,0对应第一个设备
    PCHAR iDeviceID,                        // 可选参数,指向字符串,指定被监控的设备的ID,字符串以\0终止
    mPCH347_NOTIFY_ROUTINE iNotifyRoutine); // 指定设备事件回调程序,为NULL则取消事件通知,否则在检测到事件时调用该程序

// 读取USB数据块
BOOL CH347ReadData(ULONG iIndex,     // 指定设备序号
                   PVOID oBuffer,    // 指向一个足够大的缓冲区,用于保存读取的数据
                   PULONG ioLength); // 指向长度单元,输入时为准备读取的长度,返回后为实际读取的长度

// 写取USB数据块
BOOL CH347WriteData(ULONG iIndex,     // 指定设备序号
                    PVOID iBuffer,    // 指向一个缓冲区,放置准备写出的数据
                    PULONG ioLength); // 指向长度单元,输入时为准备写出的长度,返回后为实际写出的长度

// 设置USB数据读写的超时
BOOL CH347SetTimeout(ULONG iIndex,        // 指定设备序号
                     ULONG iWriteTimeout, // 指定USB写出数据块的超时时间,以毫秒mS为单位,0xFFFFFFFF指定不超时(默认值)
                     ULONG iReadTimeout); // 指定USB读取数据块的超时时间,以毫秒mS为单位,0xFFFFFFFF指定不超时(默认值)

/***************SPI********************/
// SPI控制器初始化
BOOL CH347SPI_Init(ULONG iIndex, mSpiCfgS *SpiCfg);

//获取SPI控制器配置信息
BOOL CH347SPI_GetCfg(ULONG iIndex, mSpiCfgS *SpiCfg);

//设置片选状态,使用前需先调用CH347SPI_Init对CS进行设置
BOOL CH347SPI_ChangeCS(ULONG iIndex,   // 指定设备序号
                       UCHAR iStatus); // 0=撤消片选,1=设置片选
// 设置SPI片选
BOOL CH347SPI_SetChipSelect(ULONG iIndex,           // 指定设备序号
                            USHORT iEnableSelect,   // 低八位为CS1，高八位为CS2; 字节值为1=设置CS,为0=忽略此CS设置
                            USHORT iChipSelect,     // 低八位为CS1，高八位为CS2;片选输出,0=撤消片选,1=设置片选
                            ULONG iIsAutoDeativeCS, // 低16位为CS1，高16位为CS2;操作完成后是否自动撤消片选
                            ULONG iActiveDelay,     // 低16位为CS1，高16位为CS2;设置片选后执行读写操作的延时时间,单位us
                            ULONG iDelayDeactive);  // 低16位为CS1，高16位为CS2;撤消片选后执行读写操作的延时时间,单位us
// SPI4写数据
BOOL CH347SPI_Write(ULONG iIndex,      // 指定设备序号
                    ULONG iChipSelect, // 片选控制, 位7为0则忽略片选控制, 位7为1进行片选操作
                    ULONG iLength,     // 准备传输的数据字节数
                    ULONG iWriteStep,  // 准备读取的单个块的长度
                    PVOID ioBuffer);   // 指向一个缓冲区,放置准备从MOSI写出的数据

// SPI4读数据.无需先写数据，效率较CH347SPI_WriteRead高很多
BOOL CH347SPI_Read(ULONG iIndex,      // 指定设备序号
                   ULONG iChipSelect, // 片选控制, 位7为0则忽略片选控制, 位7为1进行片选操作
                   ULONG oLength,     // 准备发出的字节数
                   PULONG iLength,    // 准备读入的数据字节数
                   PVOID ioBuffer);   // 指向一个缓冲区,放置准备从DOUT写出的数据,返回后是从DIN读入的数据

// 处理SPI数据流,4线接口
BOOL CH347SPI_WriteRead(ULONG iIndex,      // 指定设备序号
                        ULONG iChipSelect, // 片选控制, 位7为0则忽略片选控制, 位7为1则操作片选
                        ULONG iLength,     // 准备传输的数据字节数
                        PVOID ioBuffer);   // 指向一个缓冲区,放置准备从DOUT写出的数据,返回后是从DIN读入的数据

// 处理SPI数据流,4线接口
BOOL CH347StreamSPI4(ULONG iIndex,      // 指定设备序号
                     ULONG iChipSelect, // 片选控制, 位7为0则忽略片选控制, 位7为1则参数有效
                     ULONG iLength,     // 准备传输的数据字节数
                     PVOID ioBuffer);   // 指向一个缓冲区,放置准备从DOUT写出的数据,返回后是从DIN读入的数据

/***************JTAG********************/
// JTAG接口初始化，设置模式及速度
BOOL CH347Jtag_INIT(ULONG iIndex,
                    UCHAR iClockRate); //通信速度；有效值为0-5，值越大通信速度越快

//获取Jtag速度设置
BOOL CH347Jtag_GetCfg(ULONG iIndex,      // 指定设备序号
                      UCHAR *ClockRate); //通信速度；有效值为0-5，值越大通信速度越快
//位带方式JTAG IR/DR数据读写.适用于少量数据的读写。如指令操作、状态机切换等控制类传输。如批量数据传输，建议使用CH347Jtag_WriteRead_Fast
//命令包以4096字节为单位批量读写
//状态机:Run-Test->Shift-IR/DR..->Exit IR/DR -> Run-Test
BOOL CH347Jtag_WriteRead(ULONG iIndex,          // 指定设备序号
                         BOOL IsDR,             // =TRUE: DR数据读写,=FALSE:IR数据读写
                         ULONG iWriteBitLength, // 写长度,准备写出的长度
                         PVOID iWriteBitBuffer, // 指向一个缓冲区,放置准备写出的数据
                         PULONG oReadBitLength, // 指向长度单元,返回后为实际读取的长度
                         PVOID oReadBitBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

// JTAG IR/DR数据批量读写,用于多字节连续读写。如JTAG固件下载操作。因硬件有4K缓冲区，如先写后读，长度不超过4096字节。缓冲区大小可自行调整
//状态机:Run-Test->Shift-IR/DR..->Exit IR/DR -> Run-Test
BOOL CH347Jtag_WriteRead_Fast(ULONG iIndex,          // 指定设备序号
                              BOOL IsDR,             // =TRUE: DR数据读写,=FALSE:IR数据读写
                              ULONG iWriteBitLength, // 写长度,准备写出的长度
                              PVOID iWriteBitBuffer, // 指向一个缓冲区,放置准备写出的数据
                              PULONG oReadBitLength, // 指向长度单元,返回后为实际读取的长度
                              PVOID oReadBitBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

//切换JTAG状态机
BOOL CH347Jtag_SwitchTapState(ULONG iIndex, UCHAR TapState);

// JTAG DR写,以字节为单位,用于多字节连续读写。如JTAG固件下载操作。
//状态机:Run-Test->Shift-DR..->Exit DR -> Run-Test
BOOL CH347Jtag_ByteWriteDR(ULONG iIndex,        // 指定设备序号
                           ULONG iWriteLength,  // 写长度,准备写出的字节长度
                           PVOID iWriteBuffer); // 指向一个缓冲区,放置准备写出的数据

// JTAG DR读,以字节为单位,多字节连续读。
//状态机:Run-Test->Shift-DR..->Exit DR -> Run-Test
BOOL CH347Jtag_ByteReadDR(ULONG iIndex,       // 指定设备序号
                          PULONG oReadLength, // 指向长度单元,返回后为实际读取的字节长度
                          PVOID oReadBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

// JTAG IR写,以字节为单位,多字节连续写。
//状态机:Run-Test->Shift-IR..->Exit IR -> Run-Test
BOOL CH347Jtag_ByteWriteIR(ULONG iIndex,        // 指定设备序号
                           ULONG iWriteLength,  // 写长度,准备写出的字节长度
                           PVOID iWriteBuffer); // 指向一个缓冲区,放置准备写出的数据

// JTAG IR读,以字节为单位,多字节连续读写。
//状态机:Run-Test->Shift-IR..->Exit IR -> Run-Test
BOOL CH347Jtag_ByteReadIR(ULONG iIndex,       // 指定设备序号
                          PULONG oReadLength, // 指向长度单元,返回后为实际读取的字节长度
                          PVOID oReadBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

//位带方式JTAG DR数据写.适用于少量数据的读写。如指令操作、状态机切换等控制类传输。如批量数据传输，建议使用USB20Jtag_ByeWriteDR
//状态机:Run-Test->Shift-DR..->Exit DR -> Run-Test
BOOL CH347Jtag_BitWriteDR(ULONG iIndex,           // 指定设备序号
                          ULONG iWriteBitLength,  // 指向长度单元,返回后为实际读取的字节长度
                          PVOID iWriteBitBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

//位带方式JTAG IR数据写.适用于少量数据的读写。如指令操作、状态机切换等控制类传输。如批量数据传输，建议使用USB20Jtag_ByteWriteIR
//状态机:Run-Test->Shift-IR..->Exit IR -> Run-Test
BOOL CH347Jtag_BitWriteIR(ULONG iIndex,           // 指定设备序号
                          ULONG iWriteBitLength,  // 指向长度单元,返回后为实际读取的字节长度
                          PVOID iWriteBitBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

//位带方式JTAG IR数据读.适用于少量数据的读写。如指令操作、状态机切换等。如批量数据传输，建议使用USB20Jtag_ByteReadIR
//状态机:Run-Test->Shift-IR..->Exit IR -> Run-Test
BOOL CH347Jtag_BitReadIR(ULONG iIndex,          // 指定设备序号
                         PULONG oReadBitLength, // 指向长度单元,返回后为实际读取的字节长度
                         PVOID oReadBitBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

//位带方式JTAG DR数据读.适用于少量数据的读写。如批量和高速数据传输，建议使用USB20Jtag_ByteReadDR
//状态机:Run-Test->Shift-DR..->Exit DR -> Run-Test
BOOL CH347Jtag_BitReadDR(ULONG iIndex,          // 指定设备序号
                         PULONG oReadBitLength, // 指向长度单元,返回后为实际读取的字节长度
                         PVOID oReadBitBuffer); // 指向一个足够大的缓冲区,用于保存读取的数据

//获取CH347的GPIO方向和引脚电平值
BOOL CH347GPIO_Get(ULONG iIndex,
                   UCHAR *iDir,   //引脚方向:GPIO0-7对应位0-7.0：输入；1：输出
                   UCHAR *iData); // GPIO0电平:GPIO0-7对应位0-7,0：低电平；1：高电平)

//设置CH347的GPIO方向和引脚电平值
BOOL CH347GPIO_Set(ULONG iIndex,
                   UCHAR iEnable,      //数据有效标志:对应位0-7,对应GPIO0-7.
                   UCHAR iSetDirOut,   //设置I/O方向,某位清0则对应引脚为输入,某位置1则对应引脚为输出.GPIO0-7对应位0-7.
                   UCHAR iSetDataOut); //输出数据,如果I/O方向为输出,那么某位清0时对应引脚输出低电平,某位置1时对应引脚输出高电平

// //进入IAP固件升级模式
// BOOL CH347StartIapFwUpate(ULONG iIndex,
//                           ULONG FwSize); // 固件长度


// /**************HID/VCP串口**********************/
// //打开串口
// int CH347Uart_Open(ULONG iIndex);

// //关闭串口
// BOOL CH347Uart_Close(ULONG iIndex);

// BOOL CH347Uart_SetDeviceNotify(             // 设定设备事件通知程序
//     ULONG iIndex,                           // 指定设备序号,0对应第一个设备
//     PCHAR iDeviceID,                        // 可选参数,指向字符串,指定被监控的设备的ID,字符串以\0终止
//     mPCH347_NOTIFY_ROUTINE iNotifyRoutine); // 指定设备事件回调程序,为NULL则取消事件通知,否则在检测到事件时调用该程序
// //获取UART硬件配置
// BOOL CH347Uart_GetCfg(ULONG iIndex,        // 指定设备序号
//                       PULONG BaudRate,     // 波特率
//                       PUCHAR ByteSize,     // 数据位数(5,6,7,8,16)
//                       PUCHAR Parity,       // 校验位(0：None; 1：Odd; 2：Even; 3：Mark; 4：Space)
//                       PUCHAR StopBits,     // 停止位数(0：1停止位; 1：1.5停止位; 2：2停止位)；
//                       PUCHAR ByteTimeout); //字节超时

// //设置UART配置
// BOOL CH347Uart_Init(ULONG iIndex,       // 指定设备序号
//                     DWORD BaudRate,     // 波特率
//                     UCHAR ByteSize,     // 数据位数(5,6,7,8,16)
//                     UCHAR Parity,       // 校验位(0：None; 1：Odd; 2：Even; 3：Mark; 4：Space)
//                     UCHAR StopBits,     // 停止位数(0：1停止位; 1：1.5停止位; 2：2停止位)；
//                     UCHAR ByteTimeout); // 字节超时时间,单位100uS

// // 设置USB数据读写的超时
// BOOL CH347Uart_SetTimeout(ULONG iIndex,        // 指定设备序号
//                           ULONG iWriteTimeout, // 指定USB写出数据块的超时时间,以毫秒mS为单位,0xFFFFFFFF指定不超时(默认值)
//                           ULONG iReadTimeout); // 指定USB读取数据块的超时时间,以毫秒mS为单位,0xFFFFFFFF指定不超时(默认值)

// // 读取数据块
// BOOL CH347Uart_Read(ULONG iIndex,     // 指定设备序号
//                     PVOID oBuffer,    // 指向一个足够大的缓冲区,用于保存读取的数据
//                     PULONG ioLength); // 指向长度单元,输入时为准备读取的长度,返回后为实际读取的长度
// // 写出数据块
// BOOL CH347Uart_Write(ULONG iIndex,     // 指定设备序号
//                      PVOID iBuffer,    // 指向一个缓冲区,放置准备写出的数据
//                      PULONG ioLength); // 指向长度单元,输入时为准备写出的长度,返回后为实际写出的长度

// //查询读缓冲区有多少字节未取
// BOOL CH347Uart_QueryBufUpload(ULONG iIndex, // 指定设备序号
//                               LONGLONG *RemainBytes);

// //获取设备信息
// BOOL CH347Uart_GetDeviceInfor(ULONG iIndex, mDeviceInforS *DevInformation);
 
/********IIC***********/
// 设置串口流模式
BOOL CH347I2C_Set(ULONG iIndex, // 指定设备序号
                  ULONG iMode); // 指定模式,见下行
// 位1-位0: I2C接口速度/SCL频率, 00=低速/20KHz,01=标准/100KHz(默认值),10=快速/400KHz,11=高速/750KHz
// 其它保留,必须为0

// 设置硬件异步延时,调用后很快返回,而在下一个流操作之前延时指定毫秒数
BOOL CH347I2C_SetDelaymS(ULONG iIndex,  // 指定设备序号
                         ULONG iDelay); // 指定延时的毫秒数

// 处理I2C数据流,2线接口,时钟线为SCL引脚,数据线为SDA引脚
BOOL CH347StreamI2C(ULONG iIndex,       // 指定设备序号
                    ULONG iWriteLength, // 准备写出的数据字节数
                    PVOID iWriteBuffer, // 指向一个缓冲区,放置准备写出的数据,首字节通常是I2C设备地址及读写方向位
                    ULONG iReadLength,  // 准备读取的数据字节数
                    PVOID oReadBuffer); // 指向一个缓冲区,返回后是读入的数据

typedef enum _EEPROM_TYPE
{ // EEPROM型号
    ID_24C01,
    ID_24C02,
    ID_24C04,
    ID_24C08,
    ID_24C16,
    ID_24C32,
    ID_24C64,
    ID_24C128,
    ID_24C256,
    ID_24C512,
    ID_24C1024,
    ID_24C2048,
    ID_24C4096
} EEPROM_TYPE;

// 从EEPROM中读取数据块,速度约56K字节
BOOL CH347ReadEEPROM(ULONG iIndex,          // 指定设备序号
                     EEPROM_TYPE iEepromID, // 指定EEPROM型号
                     ULONG iAddr,           // 指定数据单元的地址
                     ULONG iLength,         // 准备读取的数据字节数
                     PUCHAR oBuffer);       // 指向一个缓冲区,返回后是读入的数据
// 向EEPROM中写入数据块
BOOL CH347WriteEEPROM(ULONG iIndex,          // 指定设备序号
                      EEPROM_TYPE iEepromID, // 指定EEPROM型号
                      ULONG iAddr,           // 指定数据单元的地址
                      ULONG iLength,         // 准备写出的数据字节数
                      PUCHAR iBuffer);       // 指向一个缓冲区,放置准备写出的数据

#endif // _CH347_DLL_H
