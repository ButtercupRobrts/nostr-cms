import { Skeleton } from "@/components/ui/skeleton";
import Navigation from "@/components/Navigation";
import { cn } from "@/lib/utils";

interface PageLoadingIndicatorProps {
  showNavigation?: boolean;
  className?: string;
}

export function PageLoadingIndicator({ 
  showNavigation = true, 
  className 
}: PageLoadingIndicatorProps) {
  return (
    <div className={cn("min-h-screen bg-background", className)}>
      {showNavigation && <Navigation />}
      <div className="animate-in fade-in duration-500">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-12 space-y-8">
          {/* Hero/Header Skeleton */}
          <div className="space-y-4">
            <Skeleton className="h-12 w-3/4 max-w-2xl" />
            <Skeleton className="h-6 w-1/2 max-w-xl" />
          </div>

          {/* Content Grid Skeleton */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[...Array(6)].map((_, i) => (
              <div key={i} className="space-y-4 border rounded-xl p-6">
                <Skeleton className="h-40 w-full rounded-lg" />
                <Skeleton className="h-6 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
                <div className="space-y-2">
                  <Skeleton className="h-4 w-full" />
                  <Skeleton className="h-4 w-full" />
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
