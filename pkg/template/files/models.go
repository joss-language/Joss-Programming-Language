package files

import "path/filepath"

func GetModelFiles(path string) map[string]string {
	return map[string]string{
		filepath.Join(path, "app", "models", "auth", "User.joss"): `public class User extends Model {
    protected string $table = "users"
    protected array $fillable = [
        "first_name",
        "last_name",
        "email",
        "phone",
        "username",
        "password",
        "role_id",
        "email_verified_at"
    ]
    protected array $hidden = [
        "password"
    ]
}`,
	}
}
