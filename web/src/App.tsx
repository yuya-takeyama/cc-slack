import { Link, Outlet, useLocation } from "react-router-dom";

function App() {
  const location = useLocation();

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="container mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-6">
          cc-slack Sessions
        </h1>

        <nav className="mb-8">
          <ul className="flex space-x-6">
            <li>
              <Link
                to="/"
                className={`text-lg ${
                  location.pathname === "/web/" || location.pathname === "/web"
                    ? "text-blue-600 font-semibold"
                    : "text-gray-600 hover:text-blue-600"
                }`}
              >
                Threads
              </Link>
            </li>
            <li>
              <Link
                to="/sessions"
                className={`text-lg ${
                  location.pathname === "/web/sessions"
                    ? "text-blue-600 font-semibold"
                    : "text-gray-600 hover:text-blue-600"
                }`}
              >
                All Sessions
              </Link>
            </li>
            <li>
              <Link
                to="/manager"
                className={`text-lg ${
                  location.pathname === "/web/manager"
                    ? "text-blue-600 font-semibold"
                    : "text-gray-600 hover:text-blue-600"
                }`}
              >
                Manager
              </Link>
            </li>
          </ul>
        </nav>

        <Outlet />
      </div>
    </div>
  );
}

export default App;
