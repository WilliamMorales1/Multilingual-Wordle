package store

type Game struct {
	ID         uint          `json:"id"`
	Lang       string        `json:"lang"`
	WordLength int           `json:"word_length"`
	MaxGuesses int           `json:"max_guesses"`
	Answer     string        `json:"-"`
	Status     string        `json:"status"`
	Guesses    []GuessRecord `json:"guesses"`
}

type GuessRecord struct {
	ID      uint   `json:"id"`
	GameID  uint   `json:"game_id"`
	Attempt int    `json:"attempt"`
	Word    string `json:"word"`
	States  string `json:"states"`
}
