import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Progress } from "@/components/ui/progress";
import { UI_COPY } from "@/lib/ui-copy";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { InfoIcon } from "lucide-react";

function ComingSoonBadge() {
  return <Badge variant="secondary">{UI_COPY.comingSoon}</Badge>;
}

export function FeatureScaffold({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-6">
      <PageHeader
        title={title}
        description={description}
        actions={<ComingSoonBadge />}
      />
      {children}
    </div>
  );
}

export function FilesScaffold() {
  return (
    <FeatureScaffold
      title="File Manager"
      description="Quản lý upload, folder và metadata file."
    >
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Thư viện</CardTitle>
            <CardDescription>Upload và tổ chức file media.</CardDescription>
          </div>
          <Button disabled>+ Upload</Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Tên</TableHead>
                <TableHead>Loại</TableHead>
                <TableHead>Kích thước</TableHead>
                <TableHead>Cập nhật</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <Skeleton className="h-4 w-48" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-16" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </FeatureScaffold>
  );
}

export function VideosScaffold() {
  return (
    <FeatureScaffold
      title="Videos"
      description="Danh sách video và trạng thái chuyển mã HLS."
    >
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <Card key={i}>
            <Skeleton className="aspect-video w-full rounded-t-xl" />
            <CardHeader>
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="mt-2 h-3 w-1/2" />
            </CardHeader>
          </Card>
        ))}
      </div>
    </FeatureScaffold>
  );
}

export function VideoDetailScaffold({ id }: { id: string }) {
  return (
    <FeatureScaffold
      title="Chi tiết video"
      description={`Preview và metadata cho video ${id}.`}
    >
      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>Player</CardTitle>
          </CardHeader>
          <CardContent>
            <Skeleton className="aspect-video w-full rounded-lg" />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Metadata</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {["Trạng thái HLS", "Độ phân giải", "Thời lượng"].map((label) => (
              <div key={label}>
                <p className="text-xs text-muted-foreground">{label}</p>
                <Skeleton className="mt-1 h-4 w-full" />
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </FeatureScaffold>
  );
}

export function StorageScaffold() {
  return (
    <FeatureScaffold
      title="Storage"
      description="Dung lượng object storage và bucket."
    >
      <div className="grid gap-4 md:grid-cols-3">
        {[
          { label: "Đã dùng", value: 42 },
          { label: "Video HLS", value: 28 },
          { label: "File gốc", value: 14 },
        ].map((item) => (
          <Card key={item.label}>
            <CardHeader>
              <CardTitle className="text-sm">{item.label}</CardTitle>
            </CardHeader>
            <CardContent>
              <Progress value={item.value} className="mb-2" />
              <p className="text-xs text-muted-foreground">Dữ liệu mẫu</p>
            </CardContent>
          </Card>
        ))}
      </div>
    </FeatureScaffold>
  );
}

export function QueueScaffold() {
  return (
    <FeatureScaffold
      title="Queue"
      description="Hàng đợi chuyển mã và job nền."
    >
      <div className="grid gap-4 sm:grid-cols-3">
        {["Đang chờ", "Đang chạy", "Thất bại"].map((label) => (
          <Card key={label}>
            <CardHeader>
              <CardTitle className="text-3xl font-bold tabular-nums">—</CardTitle>
              <CardDescription>{label}</CardDescription>
            </CardHeader>
          </Card>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Jobs gần đây</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Loại</TableHead>
                <TableHead>Trạng thái</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: 4 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <Skeleton className="h-4 w-24" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-20" />
                  </TableCell>
                  <TableCell>
                    <Skeleton className="h-4 w-16" />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </FeatureScaffold>
  );
}

