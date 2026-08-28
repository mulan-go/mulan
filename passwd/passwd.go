package passwd

import (
	"math/rand"

	"golang.org/x/crypto/bcrypt"
)

var specialWord = []rune("()`!@#$%^&*_-+=|{}[]:;'<>,.?")

// Encrypt - Encrypt password to bcrypt hash
func Encrypt(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

// Verify - Verify bcrypt hash password
func Verify(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

type (
	Strength    int
	Combination int
)

const (
	Weak Strength = iota
	Low
	Medium
	High
)

const (
	Lowercase Combination = 1 << iota
	Uppercase
	Number
	Special

	Alpha = Lowercase | Uppercase
	Word  = Alpha | Number
	All   = Word | Special
)

// Check - Check password strong
func Check(password string) (Strength, Combination) {
	var (
		s Strength    = 0
		c Combination = 1
		m             = map[Combination]int{
			Lowercase: 0, Uppercase: 0, Number: 0, Special: 0,
		}
	)
	for _, p := range password {
		switch {
		case p >= '0' && p <= '9':
			m[Number]++
		case p >= 'A' && p <= 'Z':
			m[Uppercase]++
		case p >= 'a' && p <= 'z':
			m[Lowercase]++
		case p >= rune(33) && p <= rune(126):
			m[Special]++
		}
	}
	for k, v := range m {
		if v > 0 {
			s++
			c |= k
		}
	}
	return s, c
}

func Generate(length int, c Combination) string {
	passwd := make([]rune, 0, length)
	m := map[Combination]int{Lowercase: 0, Uppercase: 0, Number: 0, Special: 0}
	for len(passwd) < length {
		r := Combination(1 << rand.Intn(4))
		switch true {
		case c&r == Lowercase && m[Lowercase] < length/4:
			passwd = append(passwd, rune('a'+rand.Intn(26)))
		case c&r == Uppercase && m[Uppercase] < length/4:
			passwd = append(passwd, rune('A'+rand.Intn(26)))
		case c&r == Number && m[Number] < length/4:
			passwd = append(passwd, rune('0'+rand.Intn(10)))
		case c&r == Special && m[Special] < length/4:
			if len(passwd) == 0 {
				continue
			}
			passwd = append(passwd, specialWord[rand.Intn(28)])
		}
	}
	return string(passwd)
}
