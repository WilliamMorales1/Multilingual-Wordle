import type { AppState } from './types.js';

export const S: AppState = {
  gameId:      null,
  wordLength:  6,
  maxGuesses:  6,
  lang:        'English',
  status:      'idle',
  currentRow:  0,
  input:       [],
  charStates:  {},
  lastAttempt: 0,
  rtl:         false,
  history:     [],
};
