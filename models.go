package main

import "gorm.io/gorm"

type Game struct {
	gorm.Model
	Lang       string        `json:"lang"`
	WordLength int           `json:"word_length"`
	MaxGuesses int           `json:"max_guesses"`
	Answer     string        `json:"-"`      // never exposed to the client
	Status     string        `json:"status"` // "playing" | "won" | "lost"
	Guesses    []GuessRecord `json:"guesses" gorm:"foreignKey:GameID"`
}

type GuessRecord struct {
	gorm.Model
	GameID  uint   `json:"game_id"`
	Attempt int    `json:"attempt"`
	Word    string `json:"word"`
	States  string `json:"states"` // JSON-encoded []string, e.g. ["correct","absent","present"]
}
