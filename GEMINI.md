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

## Backup & Restore Feature Roadmap

### Phase 1: Backend API for Backups
- [x] **Step 1: Implement the Core Backup Logic in Go.**
    - This function will first ensure the backup directory `/var/www/backups/[projectName]` exists using `mkdir -p`. Then, it will perform the backup.
- [x] **Step 2: Create the `POST` API Endpoint.**
- [x_ **Step 3: Create the `GET` "List Backups" Endpoint.**

### Phase 2: Frontend UI for Managing Backups
- [x] **Step 4: Build the "Backups" Card.**
- [x] **Step 5: Add the "Create Backup" Button.**
- [x] **Step 6: Display the List of Backups.**

### Phase 3: Restore Functionality
- [ ] **Step 7: Implement the Core Restore Logic in Go.**
- [ ] **Step 8: Create the `POST` API Endpoint for Restoring.**
- [ ] **Step 9: Connect the "Restore" Button to the API.**