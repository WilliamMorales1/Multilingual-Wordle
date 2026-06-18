export interface GameResult {
  id: number;
  lang: string;
  word_length: number;
  max_guesses: number;
  status: string;
  guesses: GuessRecord[];
  alphabet: string[] | null;
  keyboard_rows: string[][] | null;
  overflow_bases: string[];
  equivalences: string[][];
  rtl: boolean;
  answer?: string;
  answer_chars?: string;
  definition?: string;
  etymology?: string;
  error?: string;
}

export interface GuessRecord {
  attempt: number;
  word: string;
  chars?: string;
  states: string[];
}

export interface GuessResult {
  attempt: number;
  word: string;
  chars?: string;
  states: string[];
  status: string;
  in_word_list: boolean;
  answer?: string;
  answer_chars?: string;
  definition?: string;
  etymology?: string;
  error?: string;
}

export interface StatsResult {
  games_played: number;
  games_won: number;
  win_pct: number;
  current_streak: number;
  max_streak: number;
  distribution: Record<string, number>;
}

export interface ProgressResult {
  count: number;
}

export interface LanguagesResult {
  languages: string[];
}

export interface NewGameRequest {
  lang: string;
  length: number;
  max_guesses: number;
}

export type GameStatus = 'idle' | 'loading' | 'playing' | 'submitting' | 'won' | 'lost';

export interface AppState {
  gameId: number | null;
  wordLength: number;
  maxGuesses: number;
  lang: string;
  status: GameStatus;
  currentRow: number;
  input: string[];
  charStates: Record<string, string>;
  lastAttempt: number;
  rtl: boolean;
}
