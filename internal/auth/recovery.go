package auth

import "strings"

// Question is one of the fixed recovery questions an administrator can pick.
type Question struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Questions are fixed so answers stay comparable across releases.
var Questions = []Question{
	{"first_pet", "What was the name of your first pet?"},
	{"birth_city", "In which city were you born?"},
	{"first_school", "What was the name of your first school?"},
	{"childhood_friend", "What is the first name of your childhood best friend?"},
	{"mother_maiden", "What is your mother's maiden name?"},
	{"first_car", "What was the make and model of your first car?"},
	{"favorite_teacher", "What was the name of your favorite teacher?"},
	{"first_job", "Where did you work for your first job?"},
}

// QuestionText returns the text of a question id.
func QuestionText(id string) (string, bool) {
	for _, q := range Questions {
		if q.ID == id {
			return q.Text, true
		}
	}
	return "", false
}

// normalizeAnswer makes answers forgiving about case and spacing.
func normalizeAnswer(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
