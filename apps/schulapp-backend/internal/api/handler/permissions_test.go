package handler

import (
	"net/http"
	"testing"

	"schulapp/internal/api"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TestSchoolCreation_OnlySuperadmin covers: superadmin can create schools;
// no other role can.
func TestSchoolCreation_OnlySuperadmin(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	superID := mustUser(t, "super@x.de", RoleSuperadmin, nil)
	schoolID := mustSchool(t, "Schule A")
	adminID := mustUser(t, "admin@a.de", RoleSchoolAdmin, &schoolID)
	teacherID := mustUser(t, "teacher@a.de", RoleTeacher, &schoolID)
	studentID := mustUser(t, "student@a.de", RoleStudent, &schoolID)

	body := api.CreateSchoolRequest{Name: "Neue Schule"}

	cases := []struct {
		name     string
		userID   int
		email    string
		role     string
		wantCode int
	}{
		{"superadmin", superID, "super@x.de", RoleSuperadmin, http.StatusCreated},
		{"school_admin", adminID, "admin@a.de", RoleSchoolAdmin, http.StatusForbidden},
		{"teacher", teacherID, "teacher@a.de", RoleTeacher, http.StatusForbidden},
		{"student", studentID, "student@a.de", RoleStudent, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := token(t, srv, tc.userID, tc.email, tc.role)
			rec := serve(srv, tok, "POST", "/api/v1/schools", body, srv.PostApiV1Schools)
			if rec.Code != tc.wantCode {
				t.Fatalf("%s: got %d, want %d, body=%s", tc.name, rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

// TestUserCreation_RoleMatrix covers: superadmin can create any user
// (including another superadmin); school_admin can only create teachers, and
// only of their own school (school_id from body is ignored/forced); teacher
// and student cannot create users at all.
func TestUserCreation_RoleMatrix(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	schoolA := mustSchool(t, "Schule A")
	schoolB := mustSchool(t, "Schule B")
	superID := mustUser(t, "super@x.de", RoleSuperadmin, nil)
	adminA := mustUser(t, "admin-a@x.de", RoleSchoolAdmin, &schoolA)
	teacherA := mustUser(t, "teacher-a@x.de", RoleTeacher, &schoolA)
	studentA := mustUser(t, "student-a@x.de", RoleStudent, &schoolA)

	t.Run("superadmin creates teacher", func(t *testing.T) {
		tok := token(t, srv, superID, "super@x.de", RoleSuperadmin)
		body := api.CreateUserRequest{Email: "new-teacher@x.de", FirstName: "A", LastName: "B", Password: "password123", Role: "teacher", SchoolId: &schoolA}
		rec := serve(srv, tok, "POST", "/api/v1/users", body, srv.PostApiV1Users)
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("superadmin creates another superadmin", func(t *testing.T) {
		tok := token(t, srv, superID, "super@x.de", RoleSuperadmin)
		body := api.CreateUserRequest{Email: "new-super@x.de", FirstName: "A", LastName: "B", Password: "password123", Role: "superadmin"}
		rec := serve(srv, tok, "POST", "/api/v1/users", body, srv.PostApiV1Users)
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("school_admin creates teacher of own school", func(t *testing.T) {
		tok := token(t, srv, adminA, "admin-a@x.de", RoleSchoolAdmin)
		body := api.CreateUserRequest{Email: "another-teacher@x.de", FirstName: "A", LastName: "B", Password: "password123", Role: "teacher", SchoolId: &schoolB}
		rec := serve(srv, tok, "POST", "/api/v1/users", body, srv.PostApiV1Users)
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
		var created api.User
		mustDecode(t, rec, &created)
		if created.SchoolId == nil || *created.SchoolId != schoolA {
			t.Fatalf("school_admin's own school_id was not enforced: got %+v, want school_id=%d (school_id in body was ignored)", created, schoolA)
		}
	})

	t.Run("school_admin cannot create school_admin or student", func(t *testing.T) {
		tok := token(t, srv, adminA, "admin-a@x.de", RoleSchoolAdmin)
		for _, role := range []api.CreateUserRequestRole{"school_admin", "student", "superadmin"} {
			body := api.CreateUserRequest{Email: openapi_types.Email("x-" + string(role) + "@x.de"), FirstName: "A", LastName: "B", Password: "password123", Role: role}
			rec := serve(srv, tok, "POST", "/api/v1/users", body, srv.PostApiV1Users)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("role=%s: got %d, want 403, body=%s", role, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("teacher and student cannot create users", func(t *testing.T) {
		for _, u := range []struct {
			id    int
			email string
			role  string
		}{{teacherA, "teacher-a@x.de", RoleTeacher}, {studentA, "student-a@x.de", RoleStudent}} {
			tok := token(t, srv, u.id, u.email, u.role)
			body := api.CreateUserRequest{Email: "nope@x.de", FirstName: "A", LastName: "B", Password: "password123", Role: "teacher"}
			rec := serve(srv, tok, "POST", "/api/v1/users", body, srv.PostApiV1Users)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("role=%s: got %d, want 403", u.role, rec.Code)
			}
		}
	})
}

// TestSchoolAdmin_CrossSchoolClassAccess_Forbidden covers: school_admin may
// manage classes of their own school, but not another school's classes.
func TestSchoolAdmin_CrossSchoolClassAccess_Forbidden(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	schoolA := mustSchool(t, "Schule A")
	schoolB := mustSchool(t, "Schule B")
	adminA := mustUser(t, "admin-a@x.de", RoleSchoolAdmin, &schoolA)
	classB := mustClass(t, schoolB, "10b")
	classA := mustClass(t, schoolA, "10a")

	tok := token(t, srv, adminA, "admin-a@x.de", RoleSchoolAdmin)

	t.Run("own school class: allowed", func(t *testing.T) {
		body := api.UpdateClassRequest{Name: strPtr("10a neu")}
		rec := serve(srv, tok, "PATCH", "/api/v1/classes/1", body, func(w http.ResponseWriter, r *http.Request) {
			srv.PatchApiV1ClassesId(w, r, classA)
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("other school class: forbidden", func(t *testing.T) {
		body := api.UpdateClassRequest{Name: strPtr("10b neu")}
		rec := serve(srv, tok, "PATCH", "/api/v1/classes/2", body, func(w http.ResponseWriter, r *http.Request) {
			srv.PatchApiV1ClassesId(w, r, classB)
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("other school class delete: forbidden", func(t *testing.T) {
		rec := serve(srv, tok, "DELETE", "/api/v1/classes/2", nil, func(w http.ResponseWriter, r *http.Request) {
			srv.DeleteApiV1ClassesId(w, r, classB)
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})
}

// TestTeacher_OnlyOwnClasses covers: a teacher may manage a class they are
// assigned to, but not a class they are not assigned to (even within the
// same school).
func TestTeacher_OnlyOwnClasses(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	school := mustSchool(t, "Schule A")
	teacherID := mustUser(t, "teacher@a.de", RoleTeacher, &school)
	otherTeacher := mustUser(t, "other@a.de", RoleTeacher, &school)
	ownClass := mustClass(t, school, "10a")
	otherClass := mustClass(t, school, "10b")
	addTeacher(t, ownClass, teacherID)
	addTeacher(t, otherClass, otherTeacher)
	subjectID := mustSubject(t)

	tok := token(t, srv, teacherID, "teacher@a.de", RoleTeacher)

	t.Run("own class: can create homework", func(t *testing.T) {
		body := api.CreateHomeworkRequest{Title: "Mathe", SubjectId: subjectID, DueDate: today()}
		rec := serve(srv, tok, "POST", "/api/v1/classes/1/homework", body, func(w http.ResponseWriter, r *http.Request) {
			srv.PostApiV1ClassesIdHomework(w, r, ownClass)
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("other teacher's class: forbidden", func(t *testing.T) {
		body := api.CreateHomeworkRequest{Title: "Mathe", SubjectId: subjectID, DueDate: today()}
		rec := serve(srv, tok, "POST", "/api/v1/classes/2/homework", body, func(w http.ResponseWriter, r *http.Request) {
			srv.PostApiV1ClassesIdHomework(w, r, otherClass)
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})
}

// TestStudent_CannotAccessAdminEndpoints covers: students get 403 on
// admin-only write endpoints.
func TestStudent_CannotAccessAdminEndpoints(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	school := mustSchool(t, "Schule A")
	studentID := mustUser(t, "student@a.de", RoleStudent, &school)
	class := mustClass(t, school, "10a")
	addMember(t, class, studentID)
	subjectID := mustSubject(t)

	tok := token(t, srv, studentID, "student@a.de", RoleStudent)

	t.Run("POST /schools", func(t *testing.T) {
		rec := serve(srv, tok, "POST", "/api/v1/schools", api.CreateSchoolRequest{Name: "X"}, srv.PostApiV1Schools)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})

	t.Run("POST /users", func(t *testing.T) {
		rec := serve(srv, tok, "POST", "/api/v1/users", api.CreateUserRequest{Email: "x@x.de", FirstName: "A", LastName: "B", Password: "password123", Role: "teacher"}, srv.PostApiV1Users)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})

	t.Run("POST /classes/:id/homework", func(t *testing.T) {
		body := api.CreateHomeworkRequest{Title: "Mathe", SubjectId: subjectID, DueDate: today()}
		rec := serve(srv, tok, "POST", "/api/v1/classes/1/homework", body, func(w http.ResponseWriter, r *http.Request) {
			srv.PostApiV1ClassesIdHomework(w, r, class)
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST /substitutions", func(t *testing.T) {
		body := api.CreateSubstitutionRequest{ClassId: &class, Date: today(), Period: 1, Type: "cancellation"}
		rec := serve(srv, tok, "POST", "/api/v1/substitutions", body, srv.PostApiV1Substitutions)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})
}

// TestHomeworkAndSubstitutions_TeacherAdminCanCreate covers the positive
// side of the same checks: teacher/admin can create homework and
// substitutions.
func TestHomeworkAndSubstitutions_TeacherAdminCanCreate(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	school := mustSchool(t, "Schule A")
	teacherID := mustUser(t, "teacher@a.de", RoleTeacher, &school)
	class := mustClass(t, school, "10a")
	addTeacher(t, class, teacherID)
	subjectID := mustSubject(t)

	tok := token(t, srv, teacherID, "teacher@a.de", RoleTeacher)

	body := api.CreateHomeworkRequest{Title: "Mathe", SubjectId: subjectID, DueDate: today()}
	rec := serve(srv, tok, "POST", "/api/v1/classes/1/homework", body, func(w http.ResponseWriter, r *http.Request) {
		srv.PostApiV1ClassesIdHomework(w, r, class)
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("homework: got %d body=%s", rec.Code, rec.Body.String())
	}

	subBody := api.CreateSubstitutionRequest{ClassId: &class, Date: today(), Period: 1, Type: "cancellation"}
	rec = serve(srv, tok, "POST", "/api/v1/substitutions", subBody, srv.PostApiV1Substitutions)
	if rec.Code != http.StatusCreated {
		t.Fatalf("substitution: got %d body=%s", rec.Code, rec.Body.String())
	}
}
