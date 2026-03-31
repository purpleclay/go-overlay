package mood

// Feeling holds the label, colour, and a light-hearted anecdote for a given score.
type Feeling struct {
	Label    string
	Color    string
	Anecdote string
}

var feelings = [12]Feeling{
	0:  {},
	1:  {"Volcanic", "#b91c1c", "You're one bad email away from a very interesting afternoon. Step away from the keyboard."},
	2:  {"Fuming", "#9333ea", "Everything is annoying right now, and you know it. Even this message is probably annoying you."},
	3:  {"Grumpy", "#7c3aed", "You have the energy of someone whose coffee went cold before they got to drink it."},
	4:  {"Irritable", "#4f46e5", "Fine. Everything's fine. (It's not fine.)"},
	5:  {"Uneasy", "#2563eb", "Something feels off but you can't quite put your finger on it. Have you had water today?"},
	6:  {"Meh", "#0284c7", "Floating through the day on a cloud of pure indifference. Deeply, profoundly okay."},
	7:  {"Okay", "#0891b2", "Not bad! You might even respond to a message today without rereading it three times."},
	8:  {"Decent", "#0d9488", "Solid day. You said good morning to someone and genuinely meant it."},
	9:  {"Bright", "#166534", "Radiating good energy. People near you are slightly confused but pleased."},
	10: {"Brilliant", "#15803d", "Suspiciously cheerful. Did something good happen or was it just a really nice snack?"},
	11: {"Transcendent", "#16a34a", "Floating six inches above the ground. Please share whatever you're having."},
}

// For returns the Feeling for a score between 1 and 11.
func For(score int) Feeling {
	if score < 1 || score > 11 {
		return Feeling{}
	}
	return feelings[score]
}

// All returns the feelings for scores 1 through 11 indexed by score.
func All() map[int]Feeling {
	result := make(map[int]Feeling, 11)
	for i := 1; i <= 11; i++ {
		result[i] = feelings[i]
	}
	return result
}
