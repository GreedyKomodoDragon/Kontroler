import { useNavigate } from "@solidjs/router";
import { useAuth } from "../providers/authProvider";

export default function Logout() {
  const { logout } = useAuth();
  const navigate = useNavigate();

  logout()
    .then((worked) => {
      if (worked) {
        navigate("/login");
        return;
      }

      navigate("/");
    })
    .catch(() => {
      navigate("/");
    });

  return <div>Logging you out...</div>;
}
