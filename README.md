# WordPress Collaboration Tool

A web-based control panel for managing WordPress instances on a Virtual Private Server (VPS). This tool simplifies the creation, deletion, and management of WordPress sites, with each site running securely in its own isolated Docker container.

## Key Features

*   **Site Management:** Create, delete, and restart WordPress sites with a single click.
*   **Containerization:** Each WordPress site is isolated in its own set of Docker containers (WordPress + MariaDB), ensuring security and stability.
*   **Plugin Management:** Easily install, activate, deactivate, and delete plugins for each individual site.
*   **Backup & Restore:** Create backups of your sites and restore them when needed.
*   **VPS Monitoring:** Keep an eye on your VPS with basic monitoring of CPU and RAM usage.
*   **Activity Log:** A comprehensive log tracks all major actions performed through the tool.
*   **Authentication:** Secure, JWT-based authentication for users.
*   **Web-based UI:** A modern and intuitive control panel built with Next.js for seamless management.

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

## Tech Stack

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

## Getting Started

### Prerequisites

*   Go (1.18+ recommended)
*   Node.js (v18.x or later)
*   pnpm (`npm install -g pnpm`)
*   A VPS with Docker, Docker Compose, and SSH access enabled.

### Configuration

The backend requires SSH credentials to connect to your VPS. Set the following environment variables:

```bash
export SSH_USER="your_ssh_username"
export SSH_HOST="your_vps_ip_address"
export SSH_PASSWORD="your_ssh_password"
```

### Installation & Running

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd wordpress-collab-tool
    ```

2.  **Run the backend:**
    Open a terminal and run:
    ```bash
    go run .
    ```
    The backend server will start on port `8080`.

3.  **Run the frontend:**
    Open a second terminal and navigate to the frontend directory:
    ```bash
    cd wordpress-control-pannel-frontend
    pnpm install
    pnpm run dev
    ```
    The frontend development server will start on port `3000`.

4.  **Access the application:**
    Open your web browser and navigate to `http://localhost:3000`.

## API Endpoints

All endpoints are prefixed with `/api`.

### Public Routes

*   `POST /login`: Authenticate a user.
*   `POST /logout`: Log out a user.

### Authenticated Routes

#### Sites
*   `GET /sites`: Get a list of all WordPress sites.
*   `POST /sites`: Create a new WordPress site.
*   `GET /sites/:projectName`: Get details for a specific site.
*   `DELETE /sites/:projectName`: Delete a site.
*   `POST /sites/:projectName/restart`: Restart a site.

#### Backups
*   `GET /sites/:projectName/backups`: List all backups for a site.
*   `POST /sites/:projectName/backups`: Create a new backup for a site.
*   `POST /sites/:projectName/backups/restore`: Restore a site from a backup file.

#### Plugins
*   `GET /sites/:projectName/plugins`: Get a list of plugins for a site.
*   `POST /sites/:projectName/plugins/:pluginName`: Install a plugin.
*   `DELETE /sites/:projectName/plugins/:pluginName`: Delete a plugin.
*   `POST /sites/:projectName/plugins/:pluginName/activate`: Activate a plugin.
*   `POST /sites/:projectName/plugins/:pluginName/deactivate`: Deactivate a plugin.

#### System
*   `GET /vps/stats`: Get CPU and RAM stats from the VPS.
*   `GET /activities`: Get a log of all activities.


## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m '''Add some AmazingFeature'''`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.
