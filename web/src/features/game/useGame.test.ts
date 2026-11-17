import { describe, expect, it } from 'vitest';
import { newer } from './useGame';
import { ApiError } from '../../api/client';
import type { Game } from '../../api/types';
const view = (version: number, phase: Game['phase'] = 'playing') => ({ id: 'room-a', version, phase } as Game);
describe('authoritative snapshots and safe errors', () => {
 it('keeps a newer socket snapshot when an earlier HTTP response arrives', () => {
  const current = view(7);
  expect(newer(current, view(5))).toBe(current);
  expect(newer(current, view(8)).version).toBe(8);
  expect(newer(undefined, current)).toBe(current);
  expect(newer(current, {...view(1),id:'room-b'}).id).toBe('room-b');
  // A storage failure may publish safe transient feedback at the committed version.
  expect(newer(current, view(7,'error')).phase).toBe('error');
 });
 it('does not expose unknown backend diagnostics in user-facing messages', () => {
  expect(new ApiError('internal process /private/engine died').message).toBe('操作暂时未能完成，请稍后重试。');
  expect(new ApiError('VERSION_CONFLICT').message).toContain('棋局已更新');
 });
});
