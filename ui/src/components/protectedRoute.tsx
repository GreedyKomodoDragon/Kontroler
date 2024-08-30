import { JSX, createEffect } from "solid-js";
import { useAuth } from "../providers/authProvider";
import { useNavigate } from "@solidjs/router";

interface ProtectedRouteProps {
  children: JSX.Element;
}

export const ProtectedRoute = (props: ProtectedRouteProps) => {
  const auth = useAuth();
  const navigate = useNavigate();

  createEffect(() => {
    if (!auth.isLoading() && !auth.isAuthenticated()) {
      navigate("/login", { replace: true });
    }
  });

  return (
    <>
      {auth.isLoading() && <div>Loading...</div>}
      {!auth.isLoading() && auth.isAuthenticated() && props.children}
    </>
  );
};
