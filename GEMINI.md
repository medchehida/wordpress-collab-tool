# Project Overview: WordPress Collaboration Tool

This project is a web-based tool for managing WordPress instances on a Virtual Private Server (VPS). It provides a control panel to create, delete, and manage WordPress sites, each running in its own Docker container.

## Key Features

*   **Site Management:** Create, delete, and restart WordPress sites.
*   **Containerization:** Each WordPress site is isolated in its own set of Docker containers (WordPress + MariaDB).
*   **Plugin Management:** Install, activate, deactivate, and delete plugins for each site.
*   **VPS Monitoring:** Basic monitoring of VPS CPU and RAM usage.
*   **Activity Log:** Tracks all major actions performed through the tool.
*   **Authentication:** JWT-based authentication for users.
*   **Web-based UI:** A Next.js-based control panel for easy management.

## Technologies Used

### Backend

*   **Language:** Go
*   **Web Framework:** Gin
*   **Key Libraries:**
    *   `github.com/gin-gonic/gin`: For the web server and API endpoints.
    *   `github.com/dgrijalva/jwt-go`: For JSON Web Token authentication.
    *   `golang.org/x/crypto/ssh` and `github.com/pkg/sftp`: For connecting to and managing the remote VPS.

### Frontend

*   **Framework:** Next.js (React)
*   **Language:** TypeScript
*   **UI Components:** `shadcn/ui` and Radix UI.
*   **Styling:** Tailwind CSS
*   **Data Fetching:** SWR
*   **Charting:** Recharts

### Infrastructure

*   **Containerization:** Docker and Docker Compose.
*   **Database:** MariaDB (for each WordPress site).

## How It Works

1.  **User Interface:** The user interacts with a Next.js frontend (the "control panel").
2.  **API Communication:** The frontend sends requests to a Go backend API.
3.  **Backend Logic:** The Go backend processes these requests.
4.  **Remote Management:** For actions like creating a site, the backend uses SSH to connect to the remote VPS.
5.  **Site Provisioning:** On the VPS, the backend:
    *   Creates a directory for the new site.
    *   Generates a `docker-compose.yml` file from a template.
    *   Runs `docker-compose up -d` to launch the WordPress and MariaDB containers.
6.  **Data Storage:**
    *   `sites.json`: Acts as a simple database to keep track of the managed sites.
    *   `activities.json`: Stores a log of all actions.
