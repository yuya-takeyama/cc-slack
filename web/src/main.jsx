import React from "react";
import ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import App from "./App.jsx";
import SessionsPage from "./pages/SessionsPage.jsx";
import ThreadSessionsPage from "./pages/ThreadSessionsPage.jsx";
import ThreadsPage from "./pages/ThreadsPage.jsx";
import "../styles/index.css";

const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <App />,
      children: [
        {
          index: true,
          element: <ThreadsPage />,
        },
        {
          path: "sessions",
          element: <SessionsPage />,
        },
        {
          path: "threads/:threadId/sessions",
          element: <ThreadSessionsPage />,
        },
      ],
    },
  ],
  {
    basename: "/web",
  },
);

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
);
