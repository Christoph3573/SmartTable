package handler

import (
	"net/http"
	"testing"

	"schulapp/internal/api"
)

// TestChat_StudentCanReachClassmateAndTeacher covers: a student can open a
// direct chat with a teacher or student who shares a class with them.
func TestChat_StudentCanReachClassmateAndTeacher(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	school := mustSchool(t, "Schule A")
	class := mustClass(t, school, "10a")
	studentA := mustUser(t, "student-a@x.de", RoleStudent, &school)
	studentB := mustUser(t, "student-b@x.de", RoleStudent, &school)
	teacher := mustUser(t, "teacher@x.de", RoleTeacher, &school)
	addMember(t, class, studentA)
	addMember(t, class, studentB)
	addTeacher(t, class, teacher)

	tok := token(t, srv, studentA, "student-a@x.de", RoleStudent)

	t.Run("direct chat with classmate", func(t *testing.T) {
		body := api.CreateChannelRequest{Type: "direct", MemberIds: &[]int{studentB}}
		rec := serve(srv, tok, "POST", "/api/v1/channels", body, srv.PostApiV1Channels)
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("direct chat with own teacher", func(t *testing.T) {
		body := api.CreateChannelRequest{Type: "direct", MemberIds: &[]int{teacher}}
		rec := serve(srv, tok, "POST", "/api/v1/channels", body, srv.PostApiV1Channels)
		if rec.Code != http.StatusCreated {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

// TestChat_CrossSchool_Forbidden covers: a student cannot reach a student or
// teacher in a different school - no shared class means no chat, regardless
// of school boundary being crossed or not, which is the actual enforcement
// mechanism (canChatWith in files_chat.go).
func TestChat_CrossSchool_Forbidden(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	schoolA := mustSchool(t, "Schule A")
	schoolB := mustSchool(t, "Schule B")
	classA := mustClass(t, schoolA, "10a")
	classB := mustClass(t, schoolB, "7b")

	studentA := mustUser(t, "student-a@x.de", RoleStudent, &schoolA)
	studentB := mustUser(t, "student-b@x.de", RoleStudent, &schoolB)
	teacherB := mustUser(t, "teacher-b@x.de", RoleTeacher, &schoolB)
	addMember(t, classA, studentA)
	addMember(t, classB, studentB)
	addTeacher(t, classB, teacherB)

	tok := token(t, srv, studentA, "student-a@x.de", RoleStudent)

	t.Run("cannot chat with student of other school", func(t *testing.T) {
		body := api.CreateChannelRequest{Type: "direct", MemberIds: &[]int{studentB}}
		rec := serve(srv, tok, "POST", "/api/v1/channels", body, srv.PostApiV1Channels)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cannot chat with teacher of other school", func(t *testing.T) {
		body := api.CreateChannelRequest{Type: "direct", MemberIds: &[]int{teacherB}}
		rec := serve(srv, tok, "POST", "/api/v1/channels", body, srv.PostApiV1Channels)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cross-school contact does not appear in contact list", func(t *testing.T) {
		rec := serve(srv, tok, "GET", "/api/v1/chat/contacts", nil, srv.GetApiV1ChatContacts)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
		}
		var contacts []api.ChatContact
		mustDecode(t, rec, &contacts)
		for _, c := range contacts {
			if c.Id == studentB || c.Id == teacherB {
				t.Fatalf("cross-school user %d leaked into contact list: %+v", c.Id, contacts)
			}
		}
	})
}

// TestChat_TeacherStudentSharedClass_MessagingWorks covers: a teacher and a
// student who share a class can create a channel and exchange messages.
func TestChat_TeacherStudentSharedClass_MessagingWorks(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	school := mustSchool(t, "Schule A")
	class := mustClass(t, school, "10a")
	teacher := mustUser(t, "teacher@x.de", RoleTeacher, &school)
	student := mustUser(t, "student@x.de", RoleStudent, &school)
	addTeacher(t, class, teacher)
	addMember(t, class, student)

	teacherTok := token(t, srv, teacher, "teacher@x.de", RoleTeacher)

	body := api.CreateChannelRequest{Type: "direct", MemberIds: &[]int{student}}
	rec := serve(srv, teacherTok, "POST", "/api/v1/channels", body, srv.PostApiV1Channels)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create channel: got %d body=%s", rec.Code, rec.Body.String())
	}
	var channel api.ChatChannel
	mustDecode(t, rec, &channel)

	msgBody := api.SendMessageRequest{Content: "Hallo!"}
	rec = serve(srv, teacherTok, "POST", "/api/v1/channels/1/messages", msgBody, func(w http.ResponseWriter, r *http.Request) {
		srv.PostApiV1ChannelsIdMessages(w, r, channel.Id)
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("teacher sends message: got %d body=%s", rec.Code, rec.Body.String())
	}

	studentTok := token(t, srv, student, "student@x.de", RoleStudent)
	rec = serve(srv, studentTok, "POST", "/api/v1/channels/1/messages", api.SendMessageRequest{Content: "Hi zurück!"}, func(w http.ResponseWriter, r *http.Request) {
		srv.PostApiV1ChannelsIdMessages(w, r, channel.Id)
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("student replies: got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = serve(srv, studentTok, "GET", "/api/v1/channels/1/messages", nil, func(w http.ResponseWriter, r *http.Request) {
		srv.GetApiV1ChannelsIdMessages(w, r, channel.Id, api.GetApiV1ChannelsIdMessagesParams{})
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("list messages: got %d body=%s", rec.Code, rec.Body.String())
	}
	var msgs []api.Message
	mustDecode(t, rec, &msgs)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d: %+v", len(msgs), msgs)
	}
}

// TestChat_OutsiderCannotJoinChannel covers: a user who is not a member of a
// channel cannot read or post messages in it, even within the same school.
func TestChat_OutsiderCannotJoinChannel(t *testing.T) {
	resetDB(t)
	srv := newTestServer(t)

	school := mustSchool(t, "Schule A")
	classShared := mustClass(t, school, "10a")
	classOther := mustClass(t, school, "10b")
	teacher := mustUser(t, "teacher@x.de", RoleTeacher, &school)
	student := mustUser(t, "student@x.de", RoleStudent, &school)
	outsider := mustUser(t, "outsider@x.de", RoleStudent, &school)
	addTeacher(t, classShared, teacher)
	addMember(t, classShared, student)
	addMember(t, classOther, outsider)

	teacherTok := token(t, srv, teacher, "teacher@x.de", RoleTeacher)
	body := api.CreateChannelRequest{Type: "direct", MemberIds: &[]int{student}}
	rec := serve(srv, teacherTok, "POST", "/api/v1/channels", body, srv.PostApiV1Channels)
	var channel api.ChatChannel
	mustDecode(t, rec, &channel)

	outsiderTok := token(t, srv, outsider, "outsider@x.de", RoleStudent)
	rec = serve(srv, outsiderTok, "GET", "/api/v1/channels/1/messages", nil, func(w http.ResponseWriter, r *http.Request) {
		srv.GetApiV1ChannelsIdMessages(w, r, channel.Id, api.GetApiV1ChannelsIdMessagesParams{})
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}
