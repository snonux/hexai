# Plan to Implement OpenRouter.ai Support

This document outlines the steps to integrate OpenRouter.ai as a new LLM provider within Hexai. The implementation will follow the existing provider pattern, using `internal/llm/openai.go` as a template due to API similarities.

## 1. Update Configuration (`internal/appconfig/config.go`)

Status: Completed - configuration structs, env overrides, and sample config updated.

The application needs to be aware of the new provider and its specific configuration settings.

-   **Add OpenRouter section to `fileConfig`:**
    -   Create a new `sectionOpenRouter` struct in `internal/appconfig/config.go` to handle settings from `config.toml`.
    -   It will contain `Model`, `BaseURL`, and `Temperature` fields.
    -   Add `OpenRouter sectionOpenRouter` to the `fileConfig` struct.

-   **Add fields to `App` struct:**
    -   Add `OpenRouterBaseURL`, `OpenRouterModel`, and `OpenRouterTemperature` to the main `App` struct.

-   **Update `loadFromEnv`:**
    -   Modify the `loadFromEnv` function in `internal/appconfig/config.go` to read `HEXAI_OPENROUTER_MODEL`, `HEXAI_OPENROUTER_BASE_URL`, and `HEXAI_OPENROUTER_TEMPERATURE` from environment variables.

-   **Update `toApp()` and `mergeProviderFields`:**
    -   Add logic to `fileConfig.toApp()` to convert the `sectionOpenRouter` into the `App` struct.
    -   Add logic to `App.mergeProviderFields()` to merge the OpenRouter configuration from different sources.

-   **Update `config.toml.example`:**
    -   Add a new `[openrouter]` section to the `config.toml.example` file to demonstrate configuration for users.

## 2. Create OpenRouter Provider (`internal/llm/openrouter.go`)

Status: Completed - provider implemented with required headers and logging.

This will be the core implementation of the OpenRouter client.

-   **Create new file `internal/llm/openrouter.go`:**
    -   This file will be a copy of `internal/llm/openai.go` initially, and then modified for OpenRouter.

-   **Define `openRouterProvider` struct:**
    -   It will implement the `llm.Client` and `llm.Streamer` interfaces.
    -   It will hold the `httpClient`, `apiKey`, `baseURL`, `defaultModel`, etc.

-   **Implement `Chat` and `ChatStream` methods:**
    -   The request logic will be adapted from the OpenAI implementation.
    -   The base URL will default to `https://openrouter.ai/api/v1` if not provided in the config.
    -   **Add the required OpenRouter headers to every request:**
        -   `Authorization: Bearer $OPENROUTER_API_KEY`
        -   `HTTP-Referer: "https://github.com/snonux/hexai"` (or another suitable value)
        -   `X-Title: "Hexai"`

-   **Model Handling:**
    -   The `model` from the `Options` will be passed directly in the JSON request body, allowing users to leverage any model available on OpenRouter.
    -   Default model now uses `openrouter/auto` so requests succeed without additional configuration.

## 3. Integrate into Provider Factory (`internal/llm/provider.go`)

Status: Completed - provider factory wiring and API key plumbing added.

The application needs to be able to select and instantiate the new provider.

-   **Update `Config` struct:**
    -   Add `OpenRouterBaseURL`, `OpenRouterModel`, and `OpenRouterTemperature` fields to the `llm.Config` struct.

-   **Update `NewFromConfig` function:**
    -   Add a new `case "openrouter":` to the `switch` statement.
    -   This case will be responsible for:
        1.  Accepting an `openRouterAPIKey` string, which will be read from the `OPENROUTER_API_KEY` environment variable by the caller.
        2.  Returning an error if the key is missing.
        3.  Instantiating the new `openRouterProvider` with the correct configuration (API key, model, base URL, temperature).

## 4. Add Tests (`internal/llm/openrouter_test.go`)

Status: Completed - streaming and header unit tests added.

-   **Create `internal/llm/openrouter_test.go`:**
    -   Add unit tests for the `openRouterProvider`.
    -   Use an HTTP mock (similar to `openai_http_test.go`) to test the `Chat` and `ChatStream` implementations without making real API calls.
    -   Verify that the correct headers (`Authorization`, `HTTP-Referer`, `X-Title`) are being sent in the mock requests.
    -   Verify that the request body is correctly formatted.

## 5. Update Documentation

Status: Completed - README and configuration docs now mention OpenRouter setup.

-   Update `docs/configuration.md` and `README.md` to include instructions on how to configure and use the OpenRouter.ai provider.
-   Explicitly mention the `OPENROUTER_API_KEY` environment variable and the new settings available in `config.toml`.
