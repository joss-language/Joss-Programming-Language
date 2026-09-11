package files

import "path/filepath"

func GetControllerFiles(path string) map[string]string {
	return map[string]string{
		filepath.Join(path, "app", "controllers", "auth", "ProfileController.joss"): `public class ProfileController {
    public func index() {
        $u = Auth::user()
        $userId = Auth::id()
        $prefix = env("PREFIX", "js_")

        // Check if MFA is active in database
        $mfaRecord = GranDB::table($prefix . "user_mfa_methods")->where("user_id", $userId)->where("is_active", 1)->first()
        $hasTOTP = (!empty($mfaRecord)) ? true : false

        $qrCode = ""
        (!$hasTOTP) ? {
            $totp = MFA::generateTOTP()
            $secret = $totp["secret"]
            Session::put("temp_2fa_secret", $secret)
            $qrCode = $totp["qr_url"]
        }

        return view("profile.index", {
            "title":       "Mi Perfil",
            "first_name":  $u->first_name,
            "last_name":   $u->last_name,
            "email":       $u->email,
            "phone":       $u->phone,
            "role_id":     $u->role_id,
            "username":    $u->username,
            "mfa_enabled": $hasTOTP,
            "qr_code":     $qrCode,
            "success":     session("success"),
            "error":       session("error")
        })
    }

    public func update() {
        $id = Auth::user()->id
        
        $data = {
            "first_name": request("first_name"),
            "last_name":  request("last_name"),
            "phone":      request("phone"),
            "password":   request("password")
        }

        // Auth::update returns true/false
        $success = Auth::update($id, $data)

        return ($success) ? redirect("/profile")->with("success", "Perfil actualizado correctamente.") : back()->with("error", "Error al actualizar el perfil.")
    }

    public func activate2FA() {
        $userId = Auth::id()
        $secret = Session::get("temp_2fa_secret")
        $code = Str::trim(request("code"))
        
        (empty($secret) || empty($code)) ? {
            return redirect("/profile")->with("error", "Código o sesión de 2FA no válida.")
        }
        
        $valid = MFA::verifyTOTP($secret, $code)
        
        return ($valid) ? {
            $prefix = env("PREFIX", "js_")
            GranDB::table($prefix . "user_mfa_methods")->insert({
                "user_id": $userId,
                "method_type": "totp",
                "secret": $secret,
                "is_active": 1
            })
            Session::forget("temp_2fa_secret")
            return redirect("/profile")->with("success", "Autenticación de dos factores (2FA) activada con éxito.")
        } : {
            return redirect("/profile")->with("error", "Código de verificación incorrecto. Intenta de nuevo.")
        }
    }

    public func deactivate2FA() {
        $userId = Auth::id()
        $prefix = env("PREFIX", "js_")
        GranDB::table($prefix . "user_mfa_methods")->where("user_id", $userId)->delete()
        return redirect("/profile")->with("success", "Autenticación de dos factores (2FA) desactivada.")
    }

    public func delete() {
        $id = Auth::user()->id
        
        // Remove account
        $success = Auth::delete($id)

        return ($success) ? {
            Auth::logout()
            return redirect("/login")->withCookie("joss_token", "")->with("success", "Tu cuenta ha sido eliminada permanentemente.")
        } : {
            return back()->with("error", "Error al eliminar la cuenta.")
        }
    }
}`,

		filepath.Join(path, "app", "controllers", "web", "HomeController.joss"): `public class HomeController {
    public func index() {
        return view("welcome", {
            "title": "Bienvenido a Joss",
            "version": JOSS_VERSION
        })
    }
}`,

		filepath.Join(path, "app", "controllers", "auth", "AuthController.joss"): `public class AuthController {
    public func showLogin() {
        (!Auth::guest()) ? { return redirect("/dashboard") }
        return view("auth.login", {"title": "Iniciar Sesión"})
    }
    
    public func showRegister() {
        (!Auth::guest()) ? { return redirect("/dashboard") }
        return view("auth.register", {"title": "Crear Cuenta"})
    }
    
    public func doLogin() {
        $email = Str::trim(request("email"))
        $password = request("password")
        
        $loginResult = Auth::login($email, $password)
        $loginResult->require2FA()
        
        return $loginResult->onSuccess(func(mixed $jwt) {
            return redirect("/dashboard")->withCookie("joss_token", $jwt)
        })->onChallenge(func(mixed $tempToken) {
            Session::put("temp_2fa_token", $tempToken)
            Session::forget("user_id")
            Session::forget("user_email")
            Session::forget("user_name")
            Session::forget("user_role")
            Session::forget("user_token")
            return redirect("/2fa/verify")
        })->onFail(func(mixed $error) {
            $verificationStatus = Auth::verificationStatus($email)
            ($verificationStatus == "unverified") ? {
                $newToken = Auth::resendVerification($email)
                ($newToken && $newToken != "already_verified") ? {
                    $link = Request::root() . "/verify/" . $newToken
                    $body = "<h1>Verifica tu cuenta</h1><a href='" . $link . "'>Verificar Cuenta</a>"
                    SmtpClient::send($email, "Verifica tu cuenta", $body)
                }
                return back()->with("error", "Cuenta no verificada. Se ha enviado un nuevo correo de verificación.")
            }
            return back()->with("error", "El correo o la contraseña son incorrectos.")
        })->response()
    }

    public func showVerify2FA() {
        $tempToken = Session::get("temp_2fa_token")
        (empty($tempToken)) ? { return redirect("/login") }
        
        return view("auth.verify_2fa", {
            "title": "Verificación 2FA",
            "error": session("error")
        })
    }

    public func doVerify2FA() {
        $tempToken = Session::get("temp_2fa_token")
        (empty($tempToken)) ? { return redirect("/login") }
        
        $code = Str::trim(request("code"))
        $finalToken = Auth::verify2FAChallenge($tempToken, $code)

        return ($finalToken) ? {
            Session::forget("temp_2fa_token")
            Session::forget("user_id")
            Session::forget("user_email")
            Session::forget("user_name")
            Session::forget("user_role")
            Session::forget("user_token")
            return redirect("/dashboard")->withCookie("joss_token", $finalToken)
        } : {
            return redirect("/2fa/verify")->with("error", "Código incorrecto o sesión expirada.")
        }
    }

    public func doRegister() {
        $data = {
            "first_name": request("first_name"),
            "last_name":  request("last_name"),
            "username":   request("username"),
            "email":      request("email"),
            "password":   request("password"),
            "phone":      request("phone")
        }
        
        // Create user - returns token on success, false on failure
        $token = Auth::create($data)
        
        return ($token) ? {
            // Send Verification Email
            $link = Request::root() . "/verify/" . $token
            $body = "<h1>Bienvenido a Joss</h1><p>Por favor verifica tu cuenta haciendo click en el siguiente enlace:</p><a href='" . $link . "'>Verificar Cuenta</a>"
            
            SmtpClient::send($data["email"], "Verifica tu cuenta", $body)
            
            return redirect("/login")->with("success", "Cuenta creada. Por favor verifica tu correo (revisa spam).")
        } : {
            return back()->with("error", "Error al crear la cuenta.")
        }
    }

    public func verify(mixed $token) {
        $verified = Auth::verify($token)
        return ($verified) ? {
            return redirect("/login")->with("success", "Cuenta verificada exitosamente. Ya puedes iniciar sesión.")
        } : {
            return redirect("/login")->with("error", "Token de verificación inválido o expirado.")
        }
    }

    public func logout() {
        Auth::logout()
        return redirect("/login")->withCookie("joss_token", "")
    }
    
    // API JWT Login
    public func apiLogin() {
        $email = request("email")
        $password = request("password")
        
        $token = Auth::attempt($email, $password)
        
        return ($token) ? {
            return json({
                "status": "success",
                "token": $token,
                "user": Auth::user()
            })
        } : {
            return json({
                "status": "error",
                "message": "Invalid credentials"
            }, 401)
        }
    }
}`,

		filepath.Join(path, "app", "controllers", "api", "ApiController.joss"): `public class ApiController {
    public func register() {
        $data = {
            "first_name": request("first_name"),
            "last_name":  request("last_name"),
            "username":   request("username"),
            "email":      request("email"),
            "password":   request("password"),
            "phone":      request("phone")
        }
        
        $token = Auth::create($data)
        
        return ($token) ? {
            return json({
                "status": "success",
                "message": "User created successfully",
                "token": $token
            }, 201)
        } : {
            return json({
                "status": "error",
                "message": "Registration failed"
            }, 400)
        }
    }

    public func login() {
        $email = request("email")
        $password = request("password")
        
        $token = Auth::attempt($email, $password)
        
        return ($token) ? {
            return json({
                "status": "success",
                "token": $token,
                "user": Auth::user()
            })
        } : {
            return json({
                "status": "error",
                "message": "Invalid credentials or not verified"
            }, 401)
        }
    }

    public func refresh() {
        $user = Auth::user()
        return ($user) ? {
            $newToken = Auth::refresh($user->id)
            return json({
                "status": "success",
                "token": $newToken
            })
        } : {
            return json({"error": "Unauthorized"}, 401)
        }
    }

    public func delete() {
        $user = Auth::user()
        return ($user) ? {
            $deleted = Auth::delete($user->id)
            return ($deleted) ? {
                 return json({"status": "success", "message": "User deleted"})
            } : {
                 return json({"error": "Failed to delete"}, 500)
            }
        } : {
            return json({"error": "Unauthorized"}, 401)
        }
    }

    public func forgotPassword() {
        $email = request("email")
        $token = Auth::forgotPassword($email)
        
        return ($token) ? {
            $link = Request::root() . "/password/reset?token=" . $token
            $body = "<h1>Recuperar Contraseña</h1><p>Has solicitado restablecer tu contraseña. Haz click aquí:</p><a href='" . $link . "'>Restablecer Contraseña</a>"
            SmtpClient::send($email, "Recuperar Contraseña", $body)

            return json({
                "status": "success",
                "message": "Si el correo existe, recibirás un enlace de recuperación."
            })
        } : {
             return json({
                "status": "success",
                "message": "Si el correo existe, recibirás un enlace de recuperación."
            })
        }
    }

    public func resetPassword() {
        $token = request("token")
        $password = request("password")

        $result = Auth::resetPassword($token, $password)

        return ($result == true) ? {
            return json({
                "status": "success",
                "message": "Contraseña restablecida correctamente"
            })
        } : {
            return json({
                "status": "error",
                "message": "Error al restablecer: " . $result
            }, 400)
        }
    }
}`,

		filepath.Join(path, "app", "controllers", "web", "DashboardController.joss"): `public class DashboardController {
    public func index() {
        $u = Auth::user()
        (!$u) ? {
            Auth::logout()
            return redirect("/login")->with("error", "Sesión no válida o usuario inexistente.")
        }

        $isAdmin = Auth::hasRole("admin")
        $roleName = ($isAdmin) ? "Administrador" : "Cliente"

        $name = ($u->name) ? $u->name : ($u->first_name . " " . $u->last_name)

        return view("dashboard.index", {
            "title":      "Dashboard",
            "user_name":  $name,
            "user_email": $u->email,
            "role":       $roleName,
            "isAdmin":    $isAdmin
        })
    }
}`,

		filepath.Join(path, "app", "controllers", "auth", "PasswordController.joss"): `public class PasswordController {
    public func showForgot() {
        return view("auth.forgot", { "title": "Recuperar Contraseña" })
    }

    public func sendResetLink() {
        $email = request("email")
        $token = Auth::forgotPassword($email)
        
        return ($token) ? {
            $link = Request::root() . "/password/reset?token=" . $token
            $body = "<h1>Recuperar Contraseña</h1><p>Has solicitado restablecer tu contraseña. Haz click aquí:</p><a href='" . $link . "'>Restablecer Contraseña</a>"
            
            SmtpClient::send($email, "Recuperación de Contraseña", $body)

            return view("auth.forgot", { 
                "success": "Se ha enviado un enlace de recuperación a tu correo."
            })
        } : {
            return view("auth.forgot", { "error": "No se pudo generar el token. Verifica el email." })
        }
    }

    public func showReset() {
        $token = request("token")
        return view("auth.reset", { "token": $token, "title": "Nueva Contraseña" })
    }

    public func resetPassword() {
        $token = request("token")
        $password = request("password")
        
        $result = Auth::resetPassword($token, $password)
        
        return ($result == true) ? {
            return redirect("/login")->withCookie("flash", "Contraseña restablecida correctamente")
        } : {
            return view("auth.reset", { 
                "token": $token, 
                "error": "Error al restablecer: " . $result 
            })
        }
    }
}`,
	}
}
