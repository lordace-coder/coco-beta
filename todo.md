create document with file upload and update document with files dont work yet, they should match the endpoints of the already hosted version

/collections/{id}/documents/batch-create current batch create endpoint should be /collections/{id}/batch/documents/create
Batch Create Documents, collections/{id}/batch/documents/update ,collections/{id}/batch/documents/delete


ADD 2FA ROUTES AND FEATURES,THOUGH SENDING OF MAILS SHOULD BE HANDLED BY THE PYTHON SERVER

ADD EMAIL VERIFICATION ENDPOINTS

/auth-collections/verify-email/send
Send Verification Email
POST
/auth-collections/verify-email/verify
Verify Email
POST
/auth-collections/verify-email/resend
Resend Verification Email

FOR AUTHENTICATION THE SERVER SHOULD ONLY VERIFY THE GOOGLE TOKEN
/auth-collections/login
App user login
GET
/auth-collections/login-google
Initiate Google OAuth login
POST
/auth-collections/signup
App user signup
GET
/auth-collections/user
Get current app user
PATCH
/auth-collections/user
Update current app user
GET
/auth-collections/users
List app users
GET
/auth-collections/users/{id}
Get app user by ID
POST
/auth-collections/verify-google-token

should be

POST
/auth-collections/login
User Login
POST
/auth-collections/signup
Create New User
GET
/auth-collections/users
List All Users
GET
/auth-collections/users/{id}
Get All Users
GET
/auth-collections/user
Get Current User Details
PATCH
/auth-collections/user
Update Current User Details
POST
/auth-collections/google-verify
Verify Google Token
POST
/auth-collections/apple-verify
Verify Apple Token
POST
/auth-collections/github-verify
Verify Github Token

also add github and apple verify endpoints

ADD PASSWORD RESET ENDPOINTS

Reset-Password
GET
/auth-collections/reset-password-page
Reset Password Page
POST
/auth-collections/forgot-password
Forgot Password
POST
/auth-collections/reset-password
Reset Password


add new feature, user online status and last seen, it should store the data in the app_user.data param


add a param to the app_user update and document update to see f overidde is true, if user sets overide to true the new data will overide the old one


A MESSAGE BROKER TO HANDLE THE REALTIME SUBS,BROADCASTS,