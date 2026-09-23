// Bundle entry for the node tests: re-exports the pieces a test drives
// directly, so the test can import one bundled ES module.
export { S } from '../src/state.js';
export { onEnter, onKeyPress, onBackspace } from '../src/game.js';
export { bounceRow, buildBoard } from '../src/board.js';
export { showStats } from '../src/ui.js';
