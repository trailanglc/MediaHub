"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createAPIKey,
  fetchAPIKeys,
  revokeAPIKey,
  type APIKey,
} from "@/lib/api/api-client";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { toast } from "sonner";
import { useState } from "react";

export function ApiKeysPanel() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [newSecret, setNewSecret] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["api-keys"],
    queryFn: fetchAPIKeys,
  });

  const createMut = useMutation({
    mutationFn: () => createAPIKey({ name, scopes: ["stream"] }),
    onSuccess: (res) => {
      setNewSecret(res.secret);
      setName("");
      toast.success("Đã tạo API key — copy secret ngay, chỉ hiển thị một lần.");
      void qc.invalidateQueries({ queryKey: ["api-keys"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const revokeMut = useMutation({
    mutationFn: (publicId: string) => revokeAPIKey(publicId),
    onSuccess: () => {
      toast.success("Đã thu hồi key");
      void qc.invalidateQueries({ queryKey: ["api-keys"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title="API Keys"
        description="Khóa truy cập stream cho website bên ngoài."
      />

      {newSecret && (
        <Alert>
          <AlertDescription>
            Secret mới (chỉ hiển thị một lần):{" "}
            <code className="break-all font-mono text-xs">{newSecret}</code>
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tạo key</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Input
            placeholder="Tên key (vd. Production)"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="max-w-xs"
          />
          <Button
            onClick={() => createMut.mutate()}
            disabled={!name.trim() || createMut.isPending}
          >
            + Tạo key
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Danh sách</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Đang tải…</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Tên</TableHead>
                  <TableHead>Scopes</TableHead>
                  <TableHead>Trạng thái</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {data?.items.map((k: APIKey) => (
                  <TableRow key={k.public_id}>
                    <TableCell>{k.name}</TableCell>
                    <TableCell className="text-xs">
                      {k.scopes.join(", ")}
                    </TableCell>
                    <TableCell>{k.status}</TableCell>
                    <TableCell>
                      {k.status === "active" && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => revokeMut.mutate(k.public_id)}
                        >
                          Thu hồi
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
