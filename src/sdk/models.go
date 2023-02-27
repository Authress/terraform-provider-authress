package authress

type Role struct {
	RoleID	 	string			`json:"roleId"`
	Name 		string			`json:"name"`
	Description string 			`json:"description,omitempty"`
	Permissions []Permission	`json:"permissions"`
}

type Permission struct {
	Action 		string 	`json:"action"`
	Allow 		bool 	`json:"allow"`
	Grant		bool	`json:"grant"`
	Delegate	bool	`json:"delegate"`
}
