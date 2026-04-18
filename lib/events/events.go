package events

type UserEventsType string
type UserEvents UserEventsType
type PostEventsType string
type LogInEventType string
type DashBoardEventType string

const (
	GET_USERS   UserEventsType     = "get_users"
	GET_USER    UserEventsType     = "get_user"
	ADD_USER    UserEventsType     = "add_user"
	UPDATE_USER UserEventsType     = "update_user"
	DELETE_USER UserEventsType     = "delete_user"
	BULK_INSERT UserEventsType     = "bulk_insert"
	BULK_DELETE UserEventsType     = " bulk_delete"
	GET_POSTS   PostEventsType     = "get_posts"
	LOGIN       LogInEventType     = "login"
	DASH_BOARD  DashBoardEventType = "dashBoard"
)
