package dice

// scriptedSource returns predetermined faces instead of random ones.
//
// The last face repeats once the script runs out, so a test states only the
// rolls it cares about rather than every die a call happens to make.
type scriptedSource struct {
	faces []int
	next  int
}

// Intn returns the next scripted face, expressed the way rand.Rand would:
// zero-based, so the caller's +1 lands on the face itself.
//
// Faces outside the die are clamped rather than producing an impossible
// result, which lets NewScripted(20) mean "as high as this die goes" whatever
// die is rolled.
func (s *scriptedSource) Intn(n int) int {
	face := s.faces[s.next]
	if s.next < len(s.faces)-1 {
		s.next++
	}

	if face > n {
		face = n
	}
	if face < 1 {
		face = 1
	}
	return face - 1
}

// NewScripted returns a Roller that produces the given faces in order.
//
// This is the difference between a reproducible test and a controllable one: a
// seeded roller repeats the same rolls but a test still has to hope they suit
// the assertion, which is how t.Skip creeps into a suite. A scripted roller
// lets the test say "this attack hits" and mean it.
//
//	hit := dice.NewScripted(20, 5) // natural 20, then 5s for the damage dice
//
// It panics with no faces, because a roller that cannot produce a roll is a
// mistake at the call site rather than a condition to handle later.
func NewScripted(faces ...int) *Roller {
	if len(faces) == 0 {
		panic("dice.NewScripted: at least one face is required")
	}

	script := make([]int, len(faces))
	copy(script, faces)
	return &Roller{src: &scriptedSource{faces: script}}
}
