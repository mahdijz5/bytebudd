# Requirements Document

## Introduction

This feature adds Ollama profile management to ByteBudd. Currently, the application uses a single Ollama host and model configured via environment variables. This feature allows admins to configure multiple named Ollama profiles — each consisting of a host URL and one or more selected models — store them in the database, and activate any number of profiles simultaneously. Regular users can then choose which active profile (and therefore which model) to use when running queries, replacing the current single-model constraint.

## Glossary

- **Admin**: A ByteBudd user with `role = "admin"`, authenticated via JWT.
- **User**: A ByteBudd user with `role = "user"`, authenticated via JWT.
- **Ollama_Host**: An HTTP/HTTPS base URL pointing to a running Ollama server instance (e.g., `http://192.168.1.99:11434`).
- **Ollama_Profile**: A named configuration record stored in the database, consisting of a display name, an Ollama_Host URL, a list of selected model names available on that host, and an active/inactive status flag.
- **Profile_Manager**: The backend service responsible for creating, reading, updating, deleting, and activating Ollama_Profiles.
- **Model_Fetcher**: The backend component that queries a given Ollama_Host's `/api/tags` endpoint to retrieve the list of available models.
- **Profile_Selector**: The frontend UI component that allows a User to choose an active Ollama_Profile and model before submitting a query.
- **Admin_Panel**: The admin-only frontend section accessible from the sidebar navigation after an Admin logs in.
- **Ollama_Config_Menu**: The page within the Admin_Panel dedicated to managing Ollama_Profiles.

---

## Requirements

### Requirement 1: Admin Access to Ollama Configuration Menu

**User Story:** As an admin, I want to see an Ollama configuration menu in the sidebar after logging in, so that I can manage Ollama profiles without exposing this functionality to regular users.

#### Acceptance Criteria

1. WHEN an Admin authenticates successfully, THE Admin_Panel SHALL display an "Ollama Profiles" navigation link in the sidebar.
2. WHEN a User with `role = "user"` authenticates successfully, THE Admin_Panel SHALL NOT display the "Ollama Profiles" navigation link in the sidebar.
3. WHEN an unauthenticated request is made to any Ollama profile management API endpoint, THE Profile_Manager SHALL return an HTTP 401 response.
4. WHEN a User with `role = "user"` makes a request to any Ollama profile management API endpoint, THE Profile_Manager SHALL return an HTTP 403 response.

---

### Requirement 2: Add an Ollama Host

**User Story:** As an admin, I want to enter an Ollama host URL and verify its connectivity, so that I can register a new host before building a profile around it.

#### Acceptance Criteria

1. WHEN an Admin submits a valid HTTP or HTTPS URL as an Ollama_Host, THE Profile_Manager SHALL attempt to connect to that host's `/api/tags` endpoint within 10 seconds.
2. WHEN the Ollama_Host responds with HTTP 200, THE Model_Fetcher SHALL return the list of available model names from the response payload.
3. IF the Ollama_Host does not respond within 10 seconds, THEN THE Profile_Manager SHALL return an error response with the message "Host unreachable: connection timed out".
4. IF the Ollama_Host responds with a non-200 HTTP status code, THEN THE Profile_Manager SHALL return an error response including the received status code.
5. IF the submitted URL does not conform to HTTP or HTTPS format, THEN THE Profile_Manager SHALL return a validation error response (including an error flag) without attempting a network connection.

---

### Requirement 3: Create an Ollama Profile

**User Story:** As an admin, I want to create a named Ollama profile by selecting models from a connected host, so that users can later choose this profile when running queries.

#### Acceptance Criteria

1. WHEN an Admin provides a profile name, a valid Ollama_Host URL, and at least one selected model name, THE Profile_Manager SHALL persist the Ollama_Profile to the database and return the created profile record.
2. WHEN an Admin attempts to create an Ollama_Profile with a name that already exists, THE Profile_Manager SHALL return an HTTP 409 response with the message "Profile name already exists".
3. WHEN an Admin attempts to create an Ollama_Profile with zero selected models, THE Profile_Manager SHALL return a validation error.
4. THE Profile_Manager SHALL store each Ollama_Profile with the following fields: `id`, `name`, `host_url`, `models` (list of strings), `is_active` (default `false`), `created_at`, `updated_at`.
5. WHEN an Ollama_Profile is created, THE Profile_Manager SHALL set `is_active` to `false` by default.

---

### Requirement 4: List Ollama Profiles

**User Story:** As an admin, I want to view all configured Ollama profiles, so that I can manage them from a single overview.

#### Acceptance Criteria

1. WHEN an Admin requests the list of Ollama_Profiles, THE Profile_Manager SHALL return all profiles ordered by `created_at` descending.
2. THE Profile_Manager SHALL include the following fields in each list item: `id`, `name`, `host_url`, `models`, `is_active`, `created_at`, `updated_at`.
3. WHEN no Ollama_Profiles exist, THE Profile_Manager SHALL return HTTP 200 with a response body of `{"message": "No profiles found"}` omitting the profiles field.
4. IF Ollama_Profiles exist in the database but cannot be retrieved due to an error, THEN THE Profile_Manager SHALL return an appropriate HTTP error status code.

---

### Requirement 5: Edit an Ollama Profile

**User Story:** As an admin, I want to modify an existing Ollama profile's name, host URL, or selected models, so that I can keep profiles up to date as my infrastructure changes.

#### Acceptance Criteria

1. WHEN an Admin explicitly submits updated fields for an existing Ollama_Profile, THE Profile_Manager SHALL update only the provided fields and persist the changes to the database.
2. WHEN an Admin explicitly submits an update that changes the `host_url` of an Ollama_Profile, THE Profile_Manager SHALL re-fetch the available models from the new host before saving.
3. WHEN an Admin attempts to rename an Ollama_Profile to a name already used by a different profile, THE Profile_Manager SHALL return an HTTP 409 response.
4. WHEN an Admin attempts to update a non-existent Ollama_Profile, THE Profile_Manager SHALL return an HTTP 404 response.
5. WHEN an Ollama_Profile is updated, THE Profile_Manager SHALL set `updated_at` to the current UTC timestamp.

---

### Requirement 6: Delete an Ollama Profile

**User Story:** As an admin, I want to delete an Ollama profile that is no longer needed, so that the profile list stays clean and relevant.

#### Acceptance Criteria

1. WHEN an Admin deletes an existing Ollama_Profile, THE Profile_Manager SHALL remove the record from the database and return HTTP 204.
2. WHEN an Admin attempts to delete a non-existent Ollama_Profile, THE Profile_Manager SHALL return an HTTP 404 response.
3. WHEN an Admin deletes an Ollama_Profile that is currently `is_active = true`, THE Profile_Manager SHALL delete the profile and return HTTP 204 without blocking the operation.

---

### Requirement 7: Activate and Deactivate Ollama Profiles

**User Story:** As an admin, I want to activate or deactivate individual Ollama profiles, so that I can control which profiles are available for users to select.

#### Acceptance Criteria

1. WHEN an Admin sets an Ollama_Profile's `is_active` to `true`, THE Profile_Manager SHALL update the profile's `is_active` field to `true` and return the updated profile.
2. WHEN an Admin sets an Ollama_Profile's `is_active` to `false`, THE Profile_Manager SHALL update the profile's `is_active` field to `false` and return the updated profile.
3. THE Profile_Manager SHALL allow multiple Ollama_Profiles to have `is_active = true` simultaneously.
4. WHEN an Admin attempts to activate a non-existent Ollama_Profile, THE Profile_Manager SHALL return an HTTP 404 response.

---

### Requirement 8: User Profile and Model Selection

**User Story:** As a user, I want to choose which active Ollama profile and model to use before submitting a query, so that I can leverage different AI models based on my needs.

#### Acceptance Criteria

1. WHEN a User opens the query interface, THE Profile_Selector SHALL display only Ollama_Profiles where `is_active = true`.
2. WHEN no Ollama_Profiles have `is_active = true`, THE Profile_Selector SHALL display a message indicating no profiles are available and disable query submission.
3. WHEN a User selects an active Ollama_Profile, THE Profile_Selector SHALL display the list of models associated with that profile for the User to choose from.
4. WHEN a User submits a query with a selected profile and model, THE Profile_Manager SHALL route the query to the correct Ollama_Host using the selected model name.
5. WHEN a User submits a query without selecting a profile and model, THE Profile_Selector SHALL prevent form submission and display a validation message.
6. WHEN only one active Ollama_Profile exists and it contains exactly one model, THE Profile_Selector SHALL pre-select that profile and model automatically.

---

### Requirement 9: Fetch Available Models from a Host

**User Story:** As an admin, I want the system to automatically retrieve the list of models available on a given Ollama host, so that I don't have to enter model names manually.

#### Acceptance Criteria

1. WHEN an Admin provides a valid Ollama_Host URL, THE Model_Fetcher SHALL query `{host_url}/api/tags` and return the list of model names from the `models[].name` fields in the response.
2. WHEN the Ollama_Host returns an empty `models` array, THE Model_Fetcher SHALL return an empty list and THE Ollama_Config_Menu SHALL display a message indicating no models are available on that host.
3. IF the Ollama_Host returns a malformed JSON response, THEN THE Model_Fetcher SHALL return an error response with the message "Invalid response from host".
4. THE Model_Fetcher SHALL be callable independently (as a dedicated API endpoint) so the Admin can refresh the model list without creating or modifying a profile.

---

### Requirement 10: Backward Compatibility with Environment-Based Configuration

**User Story:** As a developer, I want the existing environment-variable Ollama configuration to remain functional as a fallback, so that deployments without database-managed profiles continue to work.

#### Acceptance Criteria

1. WHILE no Ollama_Profiles exist in the database with `is_active = true`, THE Profile_Manager SHALL fall back to using `OLLAMA_BASE_URL` and `OLLAMA_MODEL` from the application environment configuration.
2. WHEN at least one Ollama_Profile with `is_active = true` exists in the database, THE Profile_Manager SHALL use the database-managed profiles and ignore the environment-variable configuration for user-facing queries.
3. THE Profile_Manager SHALL expose the environment configuration as a read-only "Environment Default" entry in the profile list, displaying placeholder or empty values when `OLLAMA_BASE_URL` or `OLLAMA_MODEL` are not set.
4. WHILE no active Ollama_Profiles exist and the environment variables `OLLAMA_BASE_URL` or `OLLAMA_MODEL` are missing or incomplete, THE Profile_Manager SHALL fail gracefully and return an error message indicating no Ollama configuration is available.
