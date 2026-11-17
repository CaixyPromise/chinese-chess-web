"""Real xq1 analysis contracts; fixtures are capability evidence, not strength ratings."""
import os
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'tests'))
from native import NativeClient

FEN = 'rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1'
UNIQUE_EVASION = '3k5/9/9/9/9/9/9/9/3rr4/4K4 r - - 0 1'
KING_CAPTURE = '4k4/9/9/9/9/4R4/9/9/9/4K4 r - - 0 1'
STALEMATE = '3k5/9/9/9/9/9/9/5r3/3r5/4K4 r - - 0 1'
BINARY = os.environ.get('ANALYSIS_NATIVE_BINARY')
STABLE = ('move', 'score', 'depth', 'nodes', 'evaluations', 'cache_hits', 'cutoffs', 'pv', 'score_reliable', 'stop_reason')


def search(root_move=None, depth=4, time_ms=30000, nodes=0, identity='analysis'):
    fields = f'search\tgame_id=fixture\tposition_version=7\tsearch_id={identity}\tdepth={depth}\ttime_ms={time_ms}\tnodes={nodes}\thash_mib=128'
    return fields if root_move is None else fields + '\troot_move=' + root_move


def position(client, fen=FEN, moves='', phase='auto'):
    client.call(f'position\tfen={fen}\tmoves={moves}\trules=legacy-java-v1\tphase={phase}')


class AnalysisNativeTests(unittest.TestCase):
    def client(self):
        return NativeClient(BINARY)

    def replay(self, fen, pv, moves=''):
        with self.client() as replay:
            position(replay, fen, moves)
            for move in pv.split(','):
                self.assertIn(move, replay.call('legal')['moves'].split(','))
                replay.call('apply\tmove=' + move)

    def test_capability_preserves_all_commands(self):
        with self.client() as client:
            hello = client.call('hello')
            self.assertEqual(hello['engine_version'], '0.5.2')
            self.assertEqual(hello['root_move_ready'], 'true')
            self.assertEqual(hello['commands'], 'hello,ping,reset,quit,position,legal,apply,undo,inspect,perft,evaluate,eval-tables,search,stop')

    def test_forced_root_full_iterations_identity_pv_and_fixed_config(self):
        with self.client() as client:
            position(client, moves='b0c2,b9c7', phase='end')
            before = client.call('inspect')
            result = client.call(search('h2e2'))
            self.assertEqual(result['depth'], '4')
            self.assertEqual(result['score_reliable'], 'true')
            self.assertEqual(result['move'], 'h2e2')
            self.assertTrue(client.last_info)
            for frame in client.last_info + [result]:
                self.assertEqual((frame['game_id'], frame['position_version'], frame['search_id']), ('fixture', '7', 'analysis'))
                self.assertEqual(frame['pv'].split(',')[0], 'h2e2')
            self.assertEqual(client.call('inspect'), before)
            again = client.call(search('h2e2'))
            self.assertEqual({k: result[k] for k in STABLE}, {k: again[k] for k in STABLE})
            self.replay(FEN, result['pv'], 'b0c2,b9c7')

    def test_illegal_roots_rejected_before_acceptance_without_state_change(self):
        with self.client() as client:
            position(client)
            before = client.call('inspect')
            # Malformed coordinates, empty, same-square, impossible motion,
            # opponent piece, empty origin, and self-check (unique-evasion case).
            for move in ('', 'e2', 'z0a1', 'a0a0', 'a0a9', 'a9a8', 'e4e5'):
                with self.subTest(move=move):
                    request = client.send(search(move))
                    frame_id, status, fields = client.read(5)
                    self.assertEqual((frame_id, status, fields['code']), (request, 'error', 'ILLEGAL_MOVE'))
                    self.assertEqual(client.call('inspect'), before)
            position(client, UNIQUE_EVASION)
            with self.assertRaisesRegex(RuntimeError, 'ILLEGAL_MOVE'):
                client.call(search('e0e1'))

    def test_short_budget_fallback_is_forced_and_unreliable(self):
        with self.client() as client:
            position(client)
            # One-node cap guarantees incomplete first iteration on every host.
            result = client.call(search('b0c2', depth=32, time_ms=1, nodes=1))
            self.assertEqual((result['move'], result['pv'], result['depth'], result['score_reliable'], result['stop_reason']),
                             ('b0c2', 'b0c2', '0', 'false', 'deadline'))
            self.assertEqual(client.last_info, [])

    def test_cancel_then_default_has_no_cache_or_heuristic_pollution(self):
        with self.client() as client:
            position(client)
            original = client.call(search())
            before = client.call('inspect')
            active = client.send(search('a0a2', depth=32, identity='cancel'))
            request, status, fields = client.read(5)
            self.assertEqual((request, status, fields['accepted']), (active, 'ok', 'true'))
            stop = client.send('stop\tsearch_id=cancel')
            stopped = final = None
            while stopped is None or final is None:
                request, status, fields = client.read(10)
                if request == stop:
                    self.assertEqual(status, 'ok')
                    stopped = fields
                elif request == active:
                    self.assertIn(status, ('info', 'bestmove'))
                    self.assertEqual(fields['search_id'], 'cancel')
                    if status == 'bestmove':
                        final = fields
            self.assertEqual(final['stop_reason'], 'cancelled')
            self.assertEqual(final['move'], 'a0a2')
            self.assertEqual(client.call('inspect'), before)
            subsequent = client.call(search())
            self.assertEqual({k: original[k] for k in STABLE}, {k: subsequent[k] for k in STABLE})

    def test_known_unique_evasion_and_short_terminal_pv(self):
        with self.client() as client:
            position(client, UNIQUE_EVASION)
            self.assertEqual(client.call('legal'), {'moves': 'e0f0', 'in_check': 'true'})
            result = client.call(search('e0f0', depth=3))
            self.assertEqual(result['move'], 'e0f0')
            self.assertEqual(result['score_reliable'], 'true')
            self.replay(UNIQUE_EVASION, result['pv'])
            # Minimal native terminal/capture case, not a tournament rules claim.
            position(client, KING_CAPTURE)
            result = client.call(search('e4e9', depth=3))
            self.assertEqual(result['move'], 'e4e9')
            self.assertGreater(int(result['score']), 9900)
            self.replay(KING_CAPTURE, result['pv'])
            client.call('apply\tmove=e4e9')
            terminal = client.call(search())
            self.assertEqual((terminal['move'], terminal['stop_reason'], terminal['score_reliable']), ('none', 'terminal', 'true'))
            with self.assertRaisesRegex(RuntimeError, 'ILLEGAL_MOVE'):
                client.call(search('e9e8'))
            position(client, STALEMATE)
            terminal = client.call(search())
            self.assertEqual((terminal['move'], terminal['stop_reason']), ('none', 'terminal'))
            with self.assertRaisesRegex(RuntimeError, 'ILLEGAL_MOVE'):
                client.call(search('e0e1'))



if __name__ == '__main__':
    unittest.main()
