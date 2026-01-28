import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { GripVertical, ChevronDown, ChevronUp } from "lucide-react";
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { cn } from '@/lib/utils';

interface DraggableCollapsibleCardProps {
  id: string;
  title: string;
  description?: string;
  children: React.ReactNode;
  isCollapsible?: boolean;
  defaultExpanded?: boolean;
  className?: string;
}

export function DraggableCollapsibleCard({
  id,
  title,
  description,
  children,
  isCollapsible = true,
  defaultExpanded = true,
  className
}: DraggableCollapsibleCardProps) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 50 : 'auto',
    position: 'relative' as const,
  };

  const toggleExpand = () => {
    if (isCollapsible) {
      setIsExpanded(!isExpanded);
    }
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        "group",
        isDragging && "opacity-50",
        className
      )}
    >
      <Card className="overflow-hidden border-2 border-transparent hover:border-primary/10 transition-all">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 p-4 bg-muted/30">
          <div className="flex items-center gap-3 overflow-hidden">
            <div
              {...attributes}
              {...listeners}
              className="cursor-grab active:cursor-grabbing p-1 rounded hover:bg-muted text-muted-foreground transition-colors shrink-0"
              title="Drag to reorder"
            >
              <GripVertical className="h-4 w-4" />
            </div>
            <div
              className={cn(
                "overflow-hidden cursor-pointer flex-1",
                !isCollapsible && "cursor-default"
              )}
              onClick={toggleExpand}
            >
              <CardTitle className="text-base truncate">{title}</CardTitle>
              {description && (
                <CardDescription className="text-xs truncate">{description}</CardDescription>
              )}
            </div>
          </div>

          {isCollapsible && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 w-8 p-0"
              onClick={toggleExpand}
            >
              {isExpanded ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </Button>
          )}
        </CardHeader>

        <div
          className={cn(
            "transition-all duration-200 ease-in-out origin-top",
            isExpanded ? "max-h-[2000px] opacity-100" : "max-h-0 opacity-0 pointer-events-none"
          )}
        >
          <div className="p-0">
            {children}
          </div>
        </div>
      </Card>
    </div>
  );
}
