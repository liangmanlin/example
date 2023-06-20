package mempool

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

const(
   min_size = 32
   max_size = 256*1024
   pool_size = 14
)

var pool [pool_size]sync.Pool

func init(){
   for i:=0;i<pool_size ;i++{
      size := getSize(i)
      pool[i].New = func() interface{} {
         buf := make([]byte,size)
         return &buf
      }
   }
}

func getIndex(size int) int {
   if size < min_size{
      return 0
   }
   return bits.Len32(uint32(size-1)) - 5
}

func getSize(i int) int {
   return min_size << i
}

func getCaller() string{
   str := ""
   for i:=2;i<10;i++{
      _,file,line,ok := runtime.Caller(i)
      if !ok{
         break
      }
      str += fmt.Printf("[%s:%d]",file,line)
   }
   return str
}

func BaseBuf(buf []byte,offset int) []byte {
   size := len(buf)
   cap := cap(buf)
   bufp := unsafe.Pointer(&buf)
   basep := *(*unsafe.Pointer)(bufp) // 指向底层数组
   basep = unsafe.Add(basep,-uintptr(offset))
   buf = unsafe.Slice((*byte)(basep),cap+offset)
   return buf[:size+offset]
}

type MemPoolOffsetTls struct {
   offset int
}

func NewTls(offset int) *MemPoolOffsetTls {
   return &MemPoolOffsetTls{offset: offset}
}

func (m *MemPoolOffsetTls)Malloc(size int) []byte {
   size = size+m.offset
   if size >= max_size {
      buf := make([]byte,size)
      buf[0] = 1
      return buf[m.offset:]
   }
   idx := getIndex(size)
   buf := pool[idx].Get().(*[]byte)
   tmp := *buf
   if tmp[0] != 0 {
      call := getCaller()
      fmt.Printf("ptr:%d,i:%d,cap:%d\n caller:%s",&tmp[0],tmp[0],cap(tmp),call)
   }
   tmp[0] = 1
   *buf = (*buf)[m.offset:size]
   return *buf
}

func (m *MemPoolOffsetTls) Realloc (buf []byte,size int) []byte {
   if size <= cap(buf){
      return buf[:size]
   }
   if size < max_size {
      pbuf := m.Malloc(size)
      pbuf = pbuf[:size]
      copy(m.baseBuf(pbuf),m.baseBuf(buf))
      m.Free(buf)
      return pbuf
   }
   tmp := m.baseBuf(buf)
   newBuf := append(tmp[:cap(tmp)],make([]byte,size-cap(buf))...)[m.offset:size+m.offset]
   m.Free(buf)
   return newBuf
}

func (m *MemPoolOffsetTls) Append(buf []byte,more ...byte) []byte {
   blen := len(buf)
   mlen := len(more)
   if cap(buf)-blen >= mlen {
      return append(buf,more...)
   }
   buf = m.Realloc(buf,blen+mlen)
   return append(buf[:blen],more...)
}

func (m *MemPoolOffsetTls) AppendString(buf []byte,more string) []byte {
   blen := len(buf)
   mlen := len(more)
   if cap(buf)-blen >= mlen {
      return append(buf,more...)
   }
   buf = m.Realloc(buf,blen+mlen)
   return append(buf[:blen],more...)
}

func (m *MemPoolOffsetTls) Free (buf []byte) {
   buf = m.baseBuf(buf)
   if buf[0] != 1 {
      call := getCaller()
      fmt.Errorf("ptr:%d,i:%d,cap:%d\n caller:%s",&buf[0],buf[0],cap(buf),call)
   }
   buf[0] = 0
   if cap(buf) >= max_size {
      return
   }
   idx := getIndex(cap(buf))
   pool[idx].Put(&buf)
}

func (m *MemPoolOffsetTls) baseBuf(buf []byte) []byte {
   return BaseBuf(buf,m.offset)
}
