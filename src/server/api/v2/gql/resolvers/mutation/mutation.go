package mutation_resolvers

type MutationResolver struct{}

type response struct {
	Status  int32  `json:"status"`
	Message string `json:"message"`
}

type emoteInput struct {
	ID         string    `json:"id"`
	Name       *string   `json:"name"`
	OwnerID    *string   `json:"owner_id"`
	Visibility *int32    `json:"visibility"`
	Tags       *[]string `json:"tags"`
}

type userInput struct {
	ID     string  `json:"id"`
	RoleID *string `json:"role_id"`
}
