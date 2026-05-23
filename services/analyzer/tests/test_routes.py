"""End-to-end tests for the four HTTP routes the analyzer exposes."""

import time


def _wait_for_status(client, job_id: str, *, target: set[str], attempts: int = 50):
    for _ in range(attempts):
        body = client.get(f"/scan/{job_id}").json()
        if body["status"] in target:
            return body
        time.sleep(0.05)
    raise AssertionError(f"job {job_id} never reached {target}")


class TestHealth:
    def test_returns_health_response_shape(self, client):
        response = client.get("/health")
        assert response.status_code == 200
        body = response.json()
        assert body["status"] == "ok"
        assert "adapters_available" in body
        assert body["adapters_available"] == ["builtin"]


class TestCapabilities:
    def test_for_builtin_backend_returns_capabilities(self, client):
        response = client.get("/capabilities", params={"backend": "builtin"})
        assert response.status_code == 200
        body = response.json()
        assert "async" in body
        assert body["max_concurrent_scans"] == 1

    def test_for_unknown_backend_returns_404(self, client):
        response = client.get("/capabilities", params={"backend": "zap"})
        assert response.status_code == 404


class TestSubmitScan:
    def test_returns_202_and_uuid_job_id(self, client):
        response = client.post(
            "/scan",
            json={"url": "https://example.com", "backend": "builtin"},
        )
        assert response.status_code == 202
        body = response.json()
        assert len(body["job_id"]) == 32

    def test_rejects_unknown_backend_with_422(self, client):
        response = client.post(
            "/scan",
            json={"url": "https://example.com", "backend": "shodan"},
        )
        assert response.status_code == 422

    def test_rejects_private_loopback_url_with_422(self, client):
        response = client.post(
            "/scan",
            json={"url": "http://127.0.0.1/", "backend": "builtin"},
        )
        assert response.status_code == 422

    def test_rejects_unknown_field_with_422(self, client):
        response = client.post(
            "/scan",
            json={
                "url": "https://example.com",
                "backend": "builtin",
                "rogue": "field",
            },
        )
        assert response.status_code == 422


class TestGetScan:
    def test_unknown_job_returns_404(self, client):
        response = client.get("/scan/does-not-exist")
        assert response.status_code == 404

    def test_eventually_returns_completed_status_with_stub_adapter(self, client):
        submit_response = client.post(
            "/scan",
            json={"url": "https://example.com", "backend": "builtin"},
        )
        job_id = submit_response.json()["job_id"]
        body = _wait_for_status(client, job_id, target={"completed", "failed"})
        assert body["status"] == "completed"
        assert body["url"] == "https://example.com/"
        assert body["findings"][0]["title"] == "STUB"
