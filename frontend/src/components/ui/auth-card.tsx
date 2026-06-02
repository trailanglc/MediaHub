import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function AuthCard({
  title,
  description,
  children,
  footer,
  showLogo = true,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  showLogo?: boolean;
}) {
  return (
    <Card
      className={cn(
        "w-full border-0 bg-transparent py-0 shadow-none ring-0",
        "sm:border sm:bg-card sm:py-4 sm:shadow-lg sm:ring-1 sm:ring-foreground/10",
      )}
    >
      <CardHeader className="px-0 sm:px-4">
        {showLogo && (
          <div className="mb-2 flex items-center gap-2 lg:hidden">
            <div
              className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 via-fuchsia-500 to-cyan-500 text-sm font-bold text-white"
              aria-hidden
            >
              M
            </div>
            <span className="text-sm font-semibold text-muted-foreground">
              MediaHub
            </span>
          </div>
        )}
        <CardTitle className="text-xl font-semibold tracking-tight sm:text-2xl">
          {title}
        </CardTitle>
        {description && (
          <CardDescription className="text-pretty leading-relaxed">
            {description}
          </CardDescription>
        )}
      </CardHeader>
      <CardContent className="px-0 sm:px-4">{children}</CardContent>
      {footer && (
        <CardFooter className="border-t px-0 pt-4 sm:px-4">{footer}</CardFooter>
      )}
    </Card>
  );
}
