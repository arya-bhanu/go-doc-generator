package formconst

var (
	ChoiceQuestionRadio    = `ChoiceQuestion{Type: "RADIO"}`
	ChoiceQuestionCheckbox = `ChoiceQuestion{Type: "CHECKBOX"}`
	ChoiceQuestionDropdown = `ChoiceQuestion{Type: "DROP_DOWN"}`
	ChoiceQuestionShort    = `TextQuestion{Paragraph: false}`
	ChoiceQuestionLong     = `TextQuestion{Paragraph: true}`
	ScaleQuestion          = `ScaleQuestion{}`
	DateQuestion           = `DateQuestion{}`
	TimeQuestion           = `TimeQuestion{}`
)
