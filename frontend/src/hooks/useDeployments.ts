import { useCallback, useState } from "react";
import {
  createDeployment,
  deleteDeployment,
  listDeployments,
  type Deployment,
} from "../services/api";

export function useDeployments() {
  const [items, setItems] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await listDeployments();
      setItems(data);
    } catch (err) {
      setError("Failed to load deployments");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Apply a status update in-place without a network round-trip.
  const applyStatusUpdate = useCallback((deploymentId: string, status: string) => {
    setItems((prev) =>
      prev.map((d) => (d.id === deploymentId ? { ...d, status } : d))
    );
  }, []);

  const deploy = useCallback(
    async (repository: string, branch: string, url: string): Promise<boolean> => {
      setError(null);
      try {
        const created = await createDeployment(repository, branch, url);
        setItems((prev) => [created, ...prev]);
        return true;
      } catch (err) {
        setError("Failed to create deployment");
        console.error(err);
        return false;
      }
    },
    []
  );

  const remove = useCallback(async (id: string): Promise<boolean> => {
    setError(null);
    try {
      await deleteDeployment(id);
      setItems((prev) => prev.filter((d) => d.id !== id));
      return true;
    } catch (err) {
      setError("Failed to delete deployment");
      console.error(err);
      return false;
    }
  }, []);

  return { items, loading, error, refresh, applyStatusUpdate, deploy, remove };
}
