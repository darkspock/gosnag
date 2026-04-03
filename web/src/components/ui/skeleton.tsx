import { cn } from "@/lib/utils"

export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("rounded-md bg-muted/50 animate-pulse-subtle", className)}
      {...props}
    />
  )
}

export function Spinner() {
  return (
    <div className="text-center py-12">
      <div className="inline-block h-6 w-6 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
    </div>
  )
}

export function IssueListSkeleton() {
  return (
    <div className="border rounded-lg divide-y divide-border/60 overflow-hidden">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center justify-between p-4 border-l-2 border-l-transparent">
          <div className="flex-1 space-y-2">
            <div className="flex items-center gap-2">
              <Skeleton className="h-5 w-14 rounded-full" />
              <Skeleton className="h-5 w-12 rounded-full" />
            </div>
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="h-4 w-1/3" />
          </div>
          <div className="text-right ml-4">
            <Skeleton className="h-6 w-10 ml-auto" />
            <Skeleton className="h-3 w-12 mt-1 ml-auto" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function ProjectCardsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="rounded-lg border bg-card overflow-hidden">
          <Skeleton className="h-1 w-full rounded-none" />
          <div className="p-6 space-y-2">
            <Skeleton className="h-5 w-2/3" />
            <Skeleton className="h-4 w-1/3" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function IssueDetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-64" />
      <div className="space-y-2">
        <Skeleton className="h-7 w-3/4" />
        <div className="flex gap-2">
          <Skeleton className="h-5 w-16 rounded-full" />
          <Skeleton className="h-5 w-14 rounded-full" />
          <Skeleton className="h-5 w-24" />
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-lg border bg-card p-6 space-y-2">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-5 w-36" />
          </div>
        ))}
      </div>
      <Skeleton className="h-6 w-28" />
      <div className="border rounded-lg divide-y overflow-hidden">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="p-4 space-y-2">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-1/3" />
          </div>
        ))}
      </div>
    </div>
  )
}
