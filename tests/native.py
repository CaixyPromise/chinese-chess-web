"""Small real xq1 client; supports asynchronous search without buffered-read races."""
import os
from pathlib import Path
import select
import subprocess
import time
ROOT=Path(__file__).resolve().parents[1]
class NativeClient:
 def __init__(self,binary=None):
  self.process=subprocess.Popen([str(binary or ROOT/'build/engine/xiangqi-engine')],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,bufsize=0,cwd=ROOT)
  self.buffer=bytearray();self.sequence=0;self.last_info=[]
 def __enter__(self):return self
 def __exit__(self,*_):self.close()
 def send(self,command):
  self.sequence+=1;request=str(self.sequence);data=memoryview(f'xq1\t{request}\t{command}\n'.encode('ascii'))
  while data:data=data[os.write(self.process.stdin.fileno(),data):]
  return request
 def read(self,timeout=35):
  end=time.monotonic()+timeout
  while b'\n' not in self.buffer:
   remaining=end-time.monotonic()
   if remaining<=0 or not select.select([self.process.stdout],[],[],remaining)[0]:raise TimeoutError('native frame deadline exceeded')
   data=os.read(self.process.stdout.fileno(),65536)
   if not data:raise RuntimeError('native process output ended')
   self.buffer.extend(data)
   if len(self.buffer)>1048576:raise RuntimeError('native frame too long')
  line,_,rest=self.buffer.partition(b'\n');self.buffer=bytearray(rest);p=line.decode('ascii').split('\t')
  if len(p)<3 or p[0]!='xq1':raise RuntimeError('invalid native envelope')
  fields=dict(x.split('=',1) for x in p[3:]);return p[1],p[2],fields
 def call(self,command,timeout=35):
  request=self.send(command);self.last_info=[];end=time.monotonic()+timeout;accepted=False
  while True:
   frame_id,status,fields=self.read(max(0,end-time.monotonic()))
   if frame_id!=request:raise RuntimeError('native frame identity mismatch')
   if status=='error':raise RuntimeError(str(fields))
   if status=='ok' and fields.get('accepted')=='true':accepted=True;continue
   if status=='info' and accepted:self.last_info.append(fields);continue
   if status=='bestmove' and accepted or status=='ok':return fields
   raise RuntimeError('unexpected native event')
 def close(self):
  if self.process.poll() is None:
   try:self.send('quit');self.process.communicate(timeout=2)
   except (BrokenPipeError,OSError,subprocess.TimeoutExpired):self.process.kill();self.process.communicate()
  for pipe in (self.process.stdin,self.process.stdout,self.process.stderr):
   if pipe and not pipe.closed:pipe.close()

