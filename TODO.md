# Project Todo List

This file tracks the development progress of the WordPress Control Panel project.

## Backend (Go)

### Phase 1: Core Functionality

- [ ] **Project Setup:**
    - [x] Remove the placeholder `docker-compose.yml` from the root of the project.
    - [x] Move the `template.yml` to a more appropriate directory (e.g., `templates`).
- [ ] **API Endpoints:**
    - [x] `POST /api/sites`: Create a new WordPress site.
        - [x] Generate a random password for the database.
        - [x] Generate a random port for the WordPress site.
        - [x] Create a new `docker-compose.yml` file from the template.
        - [x] Connect to the remote server via SSH.
        - [x] Create a directory for the new site on the remote server.
        - [x] Transfer the `docker-compose.yml` file to the remote server.
        - [x] Run `docker-compose up -d` to start the site.
    - [x] `GET /api/sites`: Get a list of all WordPress sites.
    - [x] `GET /api/sites/{id}`: Get details for a single WordPress site.
    - [x] `DELETE /api/sites/{id}`: Delete a WordPress site.
    - [x] `POST /api/sites/{id}/restart`: Restart a WordPress site.
- [ ] **Security:**
    - [x] Remove hardcoded SSH credentials and use environment variables instead.
    - [x] Implement proper SSH host key verification.
    - [x] Use a secure way to store and manage secrets.

### Phase 2: Advanced Features

- [ ] **Authentication:**
    - [x] `POST /api/login`: Authenticate a user.
    - [x] `POST /api/logout`: Log out a user.
    - [x] Implement JWT-based authentication.
- [ ] **Plugin Management:**
    - [x] `GET /api/sites/{id}/plugins`: Get a list of plugins for a site.
    - [x] `POST /api/sites/{id}/plugins`: Install a new plugin on a site.
    - [x] `DELETE /api/sites/{id}/plugins/{plugin-name}`: Remove a plugin from a site.
- [ ] **VPS Monitoring:**
    - [x] `GET /api/vps/stats`: Get VPS resource usage (CPU, RAM).
- [ ] **Real-time Updates:**
    - [x] Implement a mechanism (e.g., WebSockets or polling) to provide real-time updates on site creation progress.
- [ ] **Activity Log:**
    - [x] `GET /api/activities`: Get a list of recent activities.
    - [x] Log all important events (site creation, deletion, etc.).

## Frontend (Next.js)

- [x] **API Integration:**
    - [x] Connect the frontend to the Go backend API.
    - [x] Replace all mock data with data from the API.
- [x] **Authentication:**
    - [x] Implement a login page.
    - [x] Protect routes that require authentication.
- [x] **UI Improvements:**
    - [x] Add loading indicators for API calls.
    - [x] Add error handling and display error messages to the user.
    - [x] Improve the form validation.
