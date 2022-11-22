package bot

// ChatRoulettePair is a pair of participants for chat-roulette
type ChatRoulettePair struct {
	Participant string
	Partner     string
}

// PairParticipants takes a list of potential matches for a round
// of chat roulette and returns the pairings of participants.
func PairParticipants(pairings []ChatRoulettePair) map[string]string {
	matches := make(map[string]string)

	// this holds already paired participants to avoid duplication
	alreadyPaired := make(map[string]struct{})

	for _, pair := range pairings {
		if _, ok := alreadyPaired[pair.Partner]; !ok {
			if _, ok := alreadyPaired[pair.Participant]; !ok {
				if _, ok := matches[pair.Participant]; !ok {
					matches[pair.Participant] = pair.Partner

					var void struct{}
					alreadyPaired[pair.Partner] = void
					alreadyPaired[pair.Participant] = void
				}
			}
		}
	}

	return matches
}
