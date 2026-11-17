from pathlib import Path
import subprocess
import unittest
import sys

ROOT = Path(__file__).resolve().parents[1]
ENGINE = ROOT / 'build/engine/xiangqi-engine'
sys.path.insert(0,str(ROOT/'tests'))
from native import NativeClient
FEN = 'rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1'


def exchange(requests):
    result = subprocess.run([str(ENGINE)], input=requests, text=True,
                            capture_output=True, timeout=5)
    return result, [line.split('\t') for line in result.stdout.splitlines()]


class EngineProtocolTests(unittest.TestCase):
    def test_persistent_handshake_ping_reset_quit(self):
        result, lines = exchange('xq1\t1\thello\nxq1\t2\tping\nxq1\t3\treset\nxq1\t4\tquit\nxq1\t5\tping\n')
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stderr, '')
        self.assertEqual([line[1] for line in lines], ['1', '2', '3', '4'])
        self.assertTrue(all(line[2] == 'ok' for line in lines))
        self.assertIn('chess_ready=true', lines[0])
        self.assertIn('strength_verified=false', lines[0])

    def test_rules_and_bounded_native_search_available(self):
        with NativeClient() as client:
            client.call(f'position\tfen={FEN}\trules=legacy-java-v1')
            legal=client.call('legal')['moves'].split(',')
            client.call('apply\tmove=b0c2');client.call('undo')
            result=client.call('search\tgame_id=g1\tposition_version=1\tsearch_id=s1\tdepth=3\ttime_ms=100\tnodes=10000\thash_mib=16')
            self.assertIn(result['move'],legal)
            self.assertEqual((result['game_id'],result['position_version'],result['search_id']),('g1','1','s1'))
            depths=[int(frame['depth']) for frame in client.last_info]
            self.assertEqual(depths,sorted(set(depths)))
            self.assertTrue(all(frame['search_id']=='s1' for frame in client.last_info))

    def test_transactional_history_apply_undo_reset_and_missing_position(self):
        commands = ['legal', f'position\tfen={FEN}\trules=legacy-java-v1', 'inspect',
                    'apply\tmove=b2b9', 'inspect', 'undo', 'inspect',
                    f'position\tfen={FEN}\trules=legacy-java-v1\tmoves=b0c2,a9a0', 'inspect',
                    'apply\tmove=a0a9', 'inspect', 'reset', 'inspect']
        result, lines = exchange(''.join(f'xq1\t{i}\t{cmd}\n' for i, cmd in enumerate(commands)))
        self.assertEqual(result.returncode, 0)
        self.assertIn('code=NO_POSITION', lines[0])
        self.assertIn('code=NO_POSITION', lines[-1])
        self.assertEqual(lines[2][3:], lines[6][3:])
        self.assertEqual(lines[2][3:], lines[8][3:])
        self.assertEqual(lines[2][3:], lines[10][3:])
        self.assertNotEqual(lines[2][3:], lines[4][3:])
        self.assertIn('code=ILLEGAL_MOVE', lines[7])
        self.assertIn('code=ILLEGAL_MOVE', lines[9])

    def test_perft_restores_board_and_validates_depth(self):
        result, lines = exchange(f'xq1\t1\tposition\tfen={FEN}\trules=legacy-java-v1\n'
                                 'xq1\t2\tperft\tdepth=2\nxq1\t3\tperft\tdepth=4\n')
        self.assertEqual(result.returncode, 0)
        self.assertIn('nodes=1920', lines[1])
        self.assertIn('state_restored=true', lines[1])
        self.assertIn('code=INVALID_ARGUMENT', lines[2])

    def test_evaluation_read_only_overrides_and_error_recovery(self):
        commands = ['evaluate', f'position\tfen={FEN}\trules=legacy-java-v1', 'evaluate',
                    'evaluate\tphase=end', 'evaluate', 'evaluate\tphase=bogus', 'evaluate',
                    f'position\tfen={FEN}\trules=legacy-java-v1\tphase=bogus', 'evaluate']
        result, lines = exchange(''.join(f'xq1\t{i}\t{cmd}\n' for i, cmd in enumerate(commands)))
        self.assertEqual(result.returncode, 0)
        self.assertIn('code=NO_POSITION', lines[0])
        self.assertIn('code=INVALID_ARGUMENT', lines[5])
        self.assertIn('code=INVALID_ARGUMENT', lines[7])
        self.assertEqual(lines[2][3:], lines[4][3:])
        self.assertEqual(lines[2][3:], lines[6][3:])
        self.assertEqual(lines[2][3:], lines[8][3:])
        self.assertNotEqual(lines[2][3:], lines[3][3:])

    def test_bad_requests_do_not_break_next_request(self):
        invalid = [
            ('xq2\ta\tping', 'UNSUPPORTED_VERSION'),
            ('xq1\ta\tbogus', 'UNKNOWN_COMMAND'),
            ('xq1\ta\tping\tx=1', 'INVALID_ARGUMENT'),
            ('xq1\ta\tposition\tfen=bad\trules=legacy-java-v1', 'INVALID_ARGUMENT'),
            (f'xq1\ta\tposition\tfen={FEN}\trules=future', 'UNSUPPORTED_RULES'),
            (f'xq1\ta\tposition\tfen={FEN}\trules=legacy-java-v1\tmoves=a0a0', 'INVALID_ARGUMENT'),
            ('xq1\ta\tapply\tmove=z0a1', 'INVALID_ARGUMENT'),
            ('xq1\ta\tapply\tmove=a0a1\tmove=b0b1', 'INVALID_ARGUMENT'),
            ('xq1\ta\tstop\tsearch_id=s1', 'NO_ACTIVE_SEARCH'),
            ('xq1\ta\tping\r', 'INVALID_ARGUMENT'),
            ('xq1\ta\tsearch\tgame_id=g\tposition_version=18446744073709551616\tsearch_id=s\tdepth=6\ttime_ms=1\thash_mib=1', 'INVALID_ARGUMENT'),
            ('xq1\ta\tsearch\tgame_id=g\tposition_version=0\tsearch_id=s\tdepth=0\ttime_ms=1\thash_mib=1', 'INVALID_ARGUMENT'),
        ]
        for request, error in invalid:
            with self.subTest(request=request):
                result, lines = exchange(request + '\nxq1\tend\tping\n')
                self.assertEqual(result.returncode, 0)
                self.assertEqual(len(lines), 2)
                self.assertIn('code=' + error, lines[0])
                self.assertEqual(lines[1][:3], ['xq1', 'end', 'ok'])

    def test_line_limit_and_resynchronization(self):
        result, lines = exchange('x' * 65537 + '\nxq1\ta\tping\n')
        self.assertEqual(result.returncode, 0)
        self.assertIn('code=LINE_TOO_LONG', lines[0])
        self.assertEqual(lines[1][:3], ['xq1', 'a', 'ok'])

    def test_truncated_frame_not_executed(self):
        result, lines = exchange('xq1\ta\treset')
        self.assertEqual(result.returncode, 1)
        self.assertIn('code=TRUNCATED_FRAME', lines[0])


if __name__ == '__main__':
    unittest.main()
