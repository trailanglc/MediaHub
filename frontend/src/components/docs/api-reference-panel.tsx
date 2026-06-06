"use client";

import { ApiReferenceReact } from "@scalar/api-reference-react";
import "@scalar/api-reference-react/style.css";

export function ApiReferencePanel() {
  return (
    <div className="scalar-docs -mx-2 min-h-[70vh] rounded-lg border">
      <ApiReferenceReact
        configuration={{
          url: "/docs/openapi.yaml",
          theme: "default",
          layout: "modern",
          hideModels: false,
        }}
      />
    </div>
  );
}
