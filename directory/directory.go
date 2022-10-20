package directory

// "github.com/aserto-dev/go-grpc/aserto/authorizer/directory/v1"

// type Directory struct {
// 	// directory.
// 	DirectoryClient directory.ReaderClient
// }

// func (d *Directory) resolveUser(ctx context.Context, userID string) (*directory.GetObjectResponse, error) {

// 	getObjectRequst = d.DirectoryClient.ge
// 	userResponse, getUserError := d.DirectoryClient.GetObject()

// 	// if getIdentityError != nil {
// 	// 	return nil, getIdentityError
// 	// }

// 	// userResponse, getUserError := d.DirectoryClient.GetUser(ctx,
// 	// 	&directory.GetUserRequest{Id: idResponse.GetId()},
// 	// )

// 	if getUserError != nil {
// 		return nil, getUserError
// 	}

// 	return userResponse, nil

// }

// func (d *Directory) GetUser(w http.ResponseWriter, r *http.Request) {
// 	userID := mux.Vars(r)["userID"]
// 	_, err := url.QueryUnescape(userID)

// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	user, resolveUserError := d.resolveUser(r.Context(), userID)
// 	if resolveUserError != nil {
// 		http.Error(w, resolveUserError.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	w.Header().Add("Content-Type", "application/json")
// 	encodeJSONError := json.NewEncoder(w).Encode(user.Result)
// 	if encodeJSONError != nil {
// 		http.Error(w, encodeJSONError.Error(), http.StatusBadRequest)
// 		return
// 	}
// }
