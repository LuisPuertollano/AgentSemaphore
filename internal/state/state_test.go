package state

import "testing"

func TestWorst(t *testing.T) {
	cases := []struct {
		in   []State
		want State
	}{
		{nil, Green},
		{[]State{Green, Green}, Green},
		{[]State{Green, Orange}, Orange},
		{[]State{Orange, Grey}, Grey},
		{[]State{Grey, Red, Orange}, Red},
		{[]State{Green, State(42)}, Grey},
		{[]State{State(-1)}, Grey},
	}
	for _, c := range cases {
		if got := Worst(c.in...); got != c.want {
			t.Errorf("Worst(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	for s, want := range map[State]string{Green: "green", Orange: "orange", Grey: "grey", Red: "red", State(9): "grey"} {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(s), got, want)
		}
	}
}
