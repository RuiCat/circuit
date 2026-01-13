ENTRY(_start)
SECTIONS
{
  /* 开始于 0x80000000 */
  . = 0x80000000;
  .text : { *(.text) }
}
