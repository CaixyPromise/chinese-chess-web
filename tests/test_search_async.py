from pathlib import Path
import sys
import unittest
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'tests'))
from native import NativeClient
FEN='rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1'
SEARCH='search\tgame_id=g\tposition_version=9\tsearch_id=s1\tdepth=32\ttime_ms=30000\tnodes=0\thash_mib=16'

class AsyncSearchTests(unittest.TestCase):
 def start(self,client,command=SEARCH):
  request=client.send(command);frame_id,status,fields=client.read(5)
  self.assertEqual((frame_id,status,fields.get('accepted')),(request,'ok','true'))
  return request
 def test_stop_ping_busy_and_restored_board(self):
  with NativeClient() as client:
   client.call(f'position\tfen={FEN}\trules=legacy-java-v1');before=client.call('inspect')
   search=self.start(client);ping=client.send('ping');wrong=client.send('stop\tsearch_id=wrong');busy=client.send('reset');stop=client.send('stop\tsearch_id=s1')
   replies={};final=[]
   while len(replies)<4 or not final:
    request,status,fields=client.read(5)
    if request==search:
     self.assertIn(status,('info','bestmove'));self.assertEqual(fields['search_id'],'s1')
     if status=='bestmove':final.append(fields)
    else:replies[request]=(status,fields)
   self.assertEqual(replies[ping][0],'ok');self.assertEqual(replies[wrong][1]['code'],'NO_ACTIVE_SEARCH');self.assertEqual(replies[busy][1]['code'],'BUSY');self.assertEqual(replies[stop][0],'ok')
   self.assertEqual(len(final),1);self.assertEqual(final[0]['stop_reason'],'cancelled');self.assertRegex(final[0]['move'],r'^[a-i][0-9][a-i][0-9]$')
   self.assertEqual(client.call('inspect'),before)
   self.assertEqual(client.call('stop\tsearch_id=s1')['finished'],'true')
   search=self.start(client,SEARCH.replace('search_id=s1','search_id=s2'));wrong=client.send('stop\tsearch_id=s1');stop=client.send('stop\tsearch_id=s2');got={};final=[]
   while len(got)<2 or not final:
    request,status,fields=client.read(5)
    if request==search:
     if status=='bestmove':final.append(fields)
    else:got[request]=(status,fields)
   self.assertEqual(got[wrong][1]['code'],'NO_ACTIVE_SEARCH');self.assertEqual(final[0]['search_id'],'s2');self.assertEqual(final[0]['stop_reason'],'cancelled')
 def test_deadline_publishes_only_completed_iterations_and_legal_fallback(self):
  with NativeClient() as client:
   client.call(f'position\tfen={FEN}\trules=legacy-java-v1');legal=client.call('legal')['moves'].split(',')
   result=client.call(SEARCH.replace('time_ms=30000','time_ms=1'))
   self.assertEqual(result['stop_reason'],'deadline');self.assertIn(result['move'],legal)
   if result['depth']=='0':self.assertEqual(result['score_reliable'],'false')
   else:self.assertEqual(result['score'],client.last_info[-1]['score'])
 def test_quit_waits_for_one_terminal_event(self):
  with NativeClient() as client:
   client.call(f'position\tfen={FEN}\trules=legacy-java-v1');search=self.start(client);quit_id=client.send('quit');events=[]
   while True:
    request,status,fields=client.read(5)
    if request==quit_id:self.assertEqual(status,'ok');break
    if request==search and status=='bestmove':events.append(fields)
   self.assertEqual(len(events),1);self.assertEqual(events[0]['stop_reason'],'cancelled');self.assertEqual(client.process.wait(timeout=2),0)

if __name__=='__main__':unittest.main()
