export namespace application {
	
	export class AssignmentSummary {
	    assignment_id: number;
	    page_id: string;
	    student_page_web_url: string;
	    teacher_page_web_url: string;
	    title: string;
	    student_id: number;
	    student_name: string;
	    status: string;
	    // Go type: time
	    assigned_at: any;
	    correct_count: number;
	    graded_count: number;
	    total_count: number;
	
	    static createFrom(source: any = {}) {
	        return new AssignmentSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assignment_id = source["assignment_id"];
	        this.page_id = source["page_id"];
	        this.student_page_web_url = source["student_page_web_url"];
	        this.teacher_page_web_url = source["teacher_page_web_url"];
	        this.title = source["title"];
	        this.student_id = source["student_id"];
	        this.student_name = source["student_name"];
	        this.status = source["status"];
	        this.assigned_at = this.convertValues(source["assigned_at"], null);
	        this.correct_count = source["correct_count"];
	        this.graded_count = source["graded_count"];
	        this.total_count = source["total_count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChapterTarget {
	    chapter_id?: string;
	    chapter_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChapterTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapter_id = source["chapter_id"];
	        this.chapter_name = source["chapter_name"];
	    }
	}
	export class StudentSummary {
	    id: number;
	    class: string;
	    meeting_code: string;
	    space_name: string;
	    name: string;
	    cycle_start_day: number;
	    total_sessions: number;
	    total_duration_minutes: number;
	
	    static createFrom(source: any = {}) {
	        return new StudentSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.class = source["class"];
	        this.meeting_code = source["meeting_code"];
	        this.space_name = source["space_name"];
	        this.name = source["name"];
	        this.cycle_start_day = source["cycle_start_day"];
	        this.total_sessions = source["total_sessions"];
	        this.total_duration_minutes = source["total_duration_minutes"];
	    }
	}
	export class DashboardView {
	    total_students: number;
	    total_sessions: number;
	    total_duration_minutes: number;
	    students: StudentSummary[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_students = source["total_students"];
	        this.total_sessions = source["total_sessions"];
	        this.total_duration_minutes = source["total_duration_minutes"];
	        this.students = this.convertValues(source["students"], StudentSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GenerateRemediationCommand {
	    mistake_id: number;
	    multiple_choice_count: number;
	    essay_count: number;
	    model: string;
	    custom_prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerateRemediationCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mistake_id = source["mistake_id"];
	        this.multiple_choice_count = source["multiple_choice_count"];
	        this.essay_count = source["essay_count"];
	        this.model = source["model"];
	        this.custom_prompt = source["custom_prompt"];
	    }
	}
	export class GradeAssignmentCommand {
	    model: string;
	    custom_prompt: string;
	    page_id: string;
	    exercise_ids?: number[];
	
	    static createFrom(source: any = {}) {
	        return new GradeAssignmentCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.custom_prompt = source["custom_prompt"];
	        this.page_id = source["page_id"];
	        this.exercise_ids = source["exercise_ids"];
	    }
	}
	export class GradingPageFilter {
	    search: string;
	
	    static createFrom(source: any = {}) {
	        return new GradingPageFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.search = source["search"];
	    }
	}
	export class MeetingFilter {
	    // Go type: time
	    from_date: any;
	    // Go type: time
	    to_date: any;
	    min_duration_minutes: number;
	
	    static createFrom(source: any = {}) {
	        return new MeetingFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from_date = this.convertValues(source["from_date"], null);
	        this.to_date = this.convertValues(source["to_date"], null);
	        this.min_duration_minutes = source["min_duration_minutes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PublishLessonCommand {
	    draft_id: number;
	    student_id: number;
	    page_name: string;
	    student_chapter: ChapterTarget;
	    teacher_chapter: ChapterTarget;
	
	    static createFrom(source: any = {}) {
	        return new PublishLessonCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.draft_id = source["draft_id"];
	        this.student_id = source["student_id"];
	        this.page_name = source["page_name"];
	        this.student_chapter = this.convertValues(source["student_chapter"], ChapterTarget);
	        this.teacher_chapter = this.convertValues(source["teacher_chapter"], ChapterTarget);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PublishResult {
	    page_id: string;
	    workspace_id: string;
	    page_web_url: string;
	    student_page_web_url: string;
	    teacher_page_web_url: string;
	
	    static createFrom(source: any = {}) {
	        return new PublishResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page_id = source["page_id"];
	        this.workspace_id = source["workspace_id"];
	        this.page_web_url = source["page_web_url"];
	        this.student_page_web_url = source["student_page_web_url"];
	        this.teacher_page_web_url = source["teacher_page_web_url"];
	    }
	}
	export class StudentChoice {
	    id: number;
	    name: string;
	    class: string;
	    student_workspace_id?: string;
	    teacher_workspace_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new StudentChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.class = source["class"];
	        this.student_workspace_id = source["student_workspace_id"];
	        this.teacher_workspace_id = source["teacher_workspace_id"];
	    }
	}
	export class StudentDetailView {
	    student: domain.Student;
	    total_sessions: number;
	    total_duration_minutes: number;
	    meetings: domain.Meeting[];
	
	    static createFrom(source: any = {}) {
	        return new StudentDetailView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.student = this.convertValues(source["student"], domain.Student);
	        this.total_sessions = source["total_sessions"];
	        this.total_duration_minutes = source["total_duration_minutes"];
	        this.meetings = this.convertValues(source["meetings"], domain.Meeting);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StudentFilter {
	    SearchName?: string;
	    // Go type: time
	    from_date: any;
	    // Go type: time
	    to_date: any;
	    min_duration_minutes: number;
	
	    static createFrom(source: any = {}) {
	        return new StudentFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SearchName = source["SearchName"];
	        this.from_date = this.convertValues(source["from_date"], null);
	        this.to_date = this.convertValues(source["to_date"], null);
	        this.min_duration_minutes = source["min_duration_minutes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StudentMistakeGroup {
	    student_id: number;
	    student_name: string;
	    mistakes: domain.Mistake[];
	
	    static createFrom(source: any = {}) {
	        return new StudentMistakeGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.student_id = source["student_id"];
	        this.student_name = source["student_name"];
	        this.mistakes = this.convertValues(source["mistakes"], domain.Mistake);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace domain {
	
	export class AIModelInfo {
	    id: string;
	    display_name: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new AIModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.description = source["description"];
	    }
	}
	export class Mistake {
	    id: number;
	    source_assignment_id: number;
	    parent_mistake_id?: number;
	    depth: number;
	    topic: string;
	    error_reason: string;
	    status: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Mistake(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source_assignment_id = source["source_assignment_id"];
	        this.parent_mistake_id = source["parent_mistake_id"];
	        this.depth = source["depth"];
	        this.topic = source["topic"];
	        this.error_reason = source["error_reason"];
	        this.status = source["status"];
	        this.created_at = this.convertValues(source["created_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GradingResult {
	    status: string;
	    is_correct: boolean;
	    score: number;
	    feedback_html: string;
	    detected_mistake?: Mistake;
	
	    static createFrom(source: any = {}) {
	        return new GradingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.is_correct = source["is_correct"];
	        this.score = source["score"];
	        this.feedback_html = source["feedback_html"];
	        this.detected_mistake = this.convertValues(source["detected_mistake"], Mistake);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StudentAnswer {
	    text?: string;
	    choice_selection: string;
	    selected_option?: string;
	    // Go type: time
	    extracted_at: any;
	
	    static createFrom(source: any = {}) {
	        return new StudentAnswer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.choice_selection = source["choice_selection"];
	        this.selected_option = source["selected_option"];
	        this.extracted_at = this.convertValues(source["extracted_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Exercise {
	    id: number;
	    type: string;
	    topic: string;
	    difficulty: string;
	    question: string;
	    options?: string[];
	    answer: string;
	    explanation: string;
	
	    static createFrom(source: any = {}) {
	        return new Exercise(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.topic = source["topic"];
	        this.difficulty = source["difficulty"];
	        this.question = source["question"];
	        this.options = source["options"];
	        this.answer = source["answer"];
	        this.explanation = source["explanation"];
	    }
	}
	export class AssignedExercise {
	    exercise: Exercise;
	    student_answer?: StudentAnswer;
	    result?: GradingResult;
	
	    static createFrom(source: any = {}) {
	        return new AssignedExercise(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exercise = this.convertValues(source["exercise"], Exercise);
	        this.student_answer = this.convertValues(source["student_answer"], StudentAnswer);
	        this.result = this.convertValues(source["result"], GradingResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Student {
	    id: number;
	    name: string;
	    class: string;
	    meeting_code: string;
	    space_name: string;
	    cycle_start_day: number;
	    student_workspace_id?: string;
	    teacher_workspace_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new Student(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.class = source["class"];
	        this.meeting_code = source["meeting_code"];
	        this.space_name = source["space_name"];
	        this.cycle_start_day = source["cycle_start_day"];
	        this.student_workspace_id = source["student_workspace_id"];
	        this.teacher_workspace_id = source["teacher_workspace_id"];
	    }
	}
	export class Assignment {
	    id: number;
	    title: string;
	    type: string;
	    status: string;
	    // Go type: time
	    assigned_at: any;
	    assignee: Student;
	    target_page_id: string;
	    student_page_web_url: string;
	    teacher_page_web_url: string;
	    items: AssignedExercise[];
	    page_ink_image: number[];
	    origin_mistake_id?: number;
	    depth: number;
	
	    static createFrom(source: any = {}) {
	        return new Assignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.assigned_at = this.convertValues(source["assigned_at"], null);
	        this.assignee = this.convertValues(source["assignee"], Student);
	        this.target_page_id = source["target_page_id"];
	        this.student_page_web_url = source["student_page_web_url"];
	        this.teacher_page_web_url = source["teacher_page_web_url"];
	        this.items = this.convertValues(source["items"], AssignedExercise);
	        this.page_ink_image = source["page_ink_image"];
	        this.origin_mistake_id = source["origin_mistake_id"];
	        this.depth = source["depth"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Page {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Chapter {
	    id: string;
	    name: string;
	    pages: Page[];
	
	    static createFrom(source: any = {}) {
	        return new Chapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.pages = this.convertValues(source["pages"], Page);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class TeacherExample {
	    example_num: number;
	    problem: string;
	    teacher_solution: string;
	    student_friendly_explanation: string;
	    common_mistake?: string;
	
	    static createFrom(source: any = {}) {
	        return new TeacherExample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.example_num = source["example_num"];
	        this.problem = source["problem"];
	        this.teacher_solution = source["teacher_solution"];
	        this.student_friendly_explanation = source["student_friendly_explanation"];
	        this.common_mistake = source["common_mistake"];
	    }
	}
	export class Section {
	    section_title: string;
	    transition_intro: string;
	    detailed_content: string;
	    key_takeaway: string;
	    student_cloze_notes: string[];
	    teacher_examples: TeacherExample[];
	
	    static createFrom(source: any = {}) {
	        return new Section(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.section_title = source["section_title"];
	        this.transition_intro = source["transition_intro"];
	        this.detailed_content = source["detailed_content"];
	        this.key_takeaway = source["key_takeaway"];
	        this.student_cloze_notes = source["student_cloze_notes"];
	        this.teacher_examples = this.convertValues(source["teacher_examples"], TeacherExample);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Lesson {
	    title: string;
	    overview: string;
	    sections: Section[];
	    exercises: Exercise[];
	
	    static createFrom(source: any = {}) {
	        return new Lesson(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.overview = source["overview"];
	        this.sections = this.convertValues(source["sections"], Section);
	        this.exercises = this.convertValues(source["exercises"], Exercise);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LessonDraft {
	    id: number;
	    source_url: string;
	    custom_prompt: string;
	    status: string;
	    error_message?: string;
	    title: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at?: any;
	    model: string;
	    lesson_data?: Lesson;
	
	    static createFrom(source: any = {}) {
	        return new LessonDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source_url = source["source_url"];
	        this.custom_prompt = source["custom_prompt"];
	        this.status = source["status"];
	        this.error_message = source["error_message"];
	        this.title = source["title"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.model = source["model"];
	        this.lesson_data = this.convertValues(source["lesson_data"], Lesson);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Participant {
	    name: string;
	    // Go type: time
	    first_joined_at: any;
	    // Go type: time
	    last_left_at: any;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new Participant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.first_joined_at = this.convertValues(source["first_joined_at"], null);
	        this.last_left_at = this.convertValues(source["last_left_at"], null);
	        this.duration = source["duration"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Meeting {
	    id: string;
	    class: string;
	    meeting_code: string;
	    space_name: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    ended_at: any;
	    participants: Participant[];
	
	    static createFrom(source: any = {}) {
	        return new Meeting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.class = source["class"];
	        this.meeting_code = source["meeting_code"];
	        this.space_name = source["space_name"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.ended_at = this.convertValues(source["ended_at"], null);
	        this.participants = this.convertValues(source["participants"], Participant);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	export class Workspace {
	    id: string;
	    name: string;
	    chapters: Chapter[];
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.chapters = this.convertValues(source["chapters"], Chapter);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

