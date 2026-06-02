"use client";

import { FolderTreeIcon } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { FolderTree } from "@/components/files/folder-tree";

export function FolderTreeSheet({
  rootFolderId,
  currentFolderId,
  onSelect,
}: {
  rootFolderId: string;
  currentFolderId: string;
  onSelect: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="min-h-10 shrink-0 lg:hidden"
          />
        }
      >
        <FolderTreeIcon className="size-4" />
        Thư mục
      </SheetTrigger>
      <SheetContent side="left" className="w-[min(100vw-2rem,18rem)] gap-0 p-0">
        <SheetHeader className="border-b border-border px-4 py-3">
          <SheetTitle>Duyệt thư mục</SheetTitle>
        </SheetHeader>
        <div className="overflow-y-auto p-3">
          <FolderTree
            rootFolderId={rootFolderId}
            rootName="Root"
            currentFolderId={currentFolderId}
            onSelect={(id) => {
              onSelect(id);
              setOpen(false);
            }}
          />
        </div>
      </SheetContent>
    </Sheet>
  );
}
