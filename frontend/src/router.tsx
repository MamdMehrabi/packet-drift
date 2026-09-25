import { createHashRouter } from "react-router-dom";
import { Dashboard } from "./pages/Dashboard";

export const router = createHashRouter([
  {
    path: "/",
    element: <Dashboard />,
  },
]);
