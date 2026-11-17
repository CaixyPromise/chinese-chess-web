import json
from pathlib import Path
import subprocess
import unittest
ROOT=Path(__file__).resolve().parents[1]
INITIAL='rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1'
class EvaluationScoreTests(unittest.TestCase):
    def test_search_score_matches_full_diagnostic_for_corpus_and_children(self):
        cases=json.loads((ROOT/'tests/fixtures/evaluation-positions.json').read_text())['cases']
        rows=['\t'.join([c.get('phase','auto'),c.get('fen',INITIAL),','.join(c.get('moves',[]))]) for c in cases]
        result=subprocess.run(['build/engine/evaluation_score_test'],input='\n'.join(rows)+'\n',text=True,capture_output=True,cwd=ROOT,timeout=30)
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertTrue(result.stdout.startswith(str(len(cases))+' roots'),result.stdout)
        out=ROOT/'build/engine';out.mkdir(parents=True,exist_ok=True);(out/'fast-score-parity.log').write_text(result.stdout+result.stderr)

if __name__=='__main__':
    import sys
    cases=json.loads((ROOT/'tests/fixtures/evaluation-positions.json').read_text())['cases']
    rows=['\t'.join([c.get('phase','auto'),c.get('fen',INITIAL),','.join(c.get('moves',[]))]) for c in cases]
    result=subprocess.run([sys.argv[1]],input='\n'.join(rows)+'\n',text=True,capture_output=True,cwd=ROOT,timeout=45)
    print(result.stdout,end='');print(result.stderr,end='',file=sys.stderr)
    if result.returncode or not result.stdout.startswith(str(len(cases))+' roots'):raise SystemExit(result.returncode or 1)
