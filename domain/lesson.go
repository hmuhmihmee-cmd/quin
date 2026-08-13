package domain

type TeacherExample struct {
	ExampleNum                 int    `json:"example_num"`
	Problem                    string `json:"problem"`
	TeacherSolution            string `json:"teacher_solution"`
	StudentFriendlyExplanation string `json:"student_friendly_explanation"`
	CommonMistake              string `json:"common_mistake,omitempty"`
}

type Section struct {
	SectionTitle      string           `json:"section_title"`
	TransitionIntro   string           `json:"transition_intro"`
	DetailedContent   string           `json:"detailed_content"`
	KeyTakeaway       string           `json:"key_takeaway"`
	StudentClozeNotes []string         `json:"student_cloze_notes"`
	TeacherExamples   []TeacherExample `json:"teacher_examples"`
}

type AIExercise struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Difficulty  string   `json:"difficulty"`
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
}

type LessonData struct {
	LessonTitle          string       `json:"lesson_title"`
	LessonOverview       string       `json:"lesson_overview"`
	Sections             []Section    `json:"sections"`
	AIGeneratedExercises []AIExercise `json:"ai_generated_exercises"`
}

type LessonFiles struct {
	Title      string
	TeacherPDF []byte
	StudentPDF []byte
}

type LessonDraft struct {
	Title           string
	SourceURL       string
	TeacherMarkdown string
	StudentMarkdown string
}
