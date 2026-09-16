package assignment

type VerdictOutcome string

const (
	OutcomeCorrect               VerdictOutcome = "CORRECT"                 // Đúng hoàn hảo
	OutcomeCorrectMissingWorking VerdictOutcome = "CORRECT_MISSING_WORKING" // Đúng đáp án nhưng thiếu bài giải
	OutcomeLuckyGuess            VerdictOutcome = "LUCKY_GUESS"             // Khoanh bừa ăn may (Khoanh đúng, giải sai)
	OutcomeMisclick              VerdictOutcome = "MISCLICK"                // Khoanh nhầm (Giải đúng, khoanh sai)
	OutcomeIncorrect             VerdictOutcome = "INCORRECT"               // Sai toàn diện
	OutcomeSkipped               VerdictOutcome = "SKIPPED"                 // Bỏ trống không làm
	OutcomeInvalidSelection      VerdictOutcome = "INVALID_SELECTION"       // Khoanh 2 đáp án
)
