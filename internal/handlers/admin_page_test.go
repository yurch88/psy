package handlers

import "testing"

func TestAdminDateSlotOptionsUseTwentyFiveMinuteStep(t *testing.T) {
	options := adminDateSlotOptions()
	if len(options) == 0 {
		t.Fatal("expected date slot options")
	}

	if options[0].Value != "09:00" || options[0].End != "09:55" {
		t.Fatalf("unexpected first option: %+v", options[0])
	}

	if options[len(options)-1].Value != "21:30" || options[len(options)-1].End != "22:25" {
		t.Fatalf("unexpected last option: %+v", options[len(options)-1])
	}

	for i := 1; i < len(options); i++ {
		prev := options[i-1]
		curr := options[i]
		if diff := minutes(curr.Value) - minutes(prev.Value); diff != 25 {
			t.Fatalf("expected 25 minute step between %s and %s, got %d", prev.Value, curr.Value, diff)
		}
	}
}

func minutes(value string) int {
	return int(value[0]-'0')*600 + int(value[1]-'0')*60 + int(value[3]-'0')*10 + int(value[4]-'0')
}
