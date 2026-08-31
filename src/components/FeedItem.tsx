import { useState, useMemo } from 'react';
import { type NostrEvent } from '@nostrify/nostrify';
import { nip19 } from 'nostr-tools';
import { formatDistanceToNow } from 'date-fns';
import {
  MessageCircle,
  Heart,
  Bookmark,
  MoreHorizontal,
  Share2,
  Copy,
  Code,
  Repeat2,
  Zap,
  Loader2,
} from 'lucide-react';

import { useAuthor } from '@/hooks/useAuthor';
import { useCurrentUser } from '@/hooks/useCurrentUser';
import { useNostrPublish } from '@/hooks/useNostrPublish';
import { useAppContext } from '@/hooks/useAppContext';
import { useDefaultRelay } from '@/hooks/useDefaultRelay';
import { useCreateRepost } from '@/hooks/useCreateRepost';
import { useToast } from '@/hooks/useToast';
import { useQueryClient } from '@tanstack/react-query';
import { genUserName } from '@/lib/genUserName';
import { NoteContent } from './NoteContent';
import { ZapDialog } from './ZapDialog';
import { Avatar, AvatarFallback, AvatarImage } from './ui/avatar';
import { Button } from './ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from './ui/dropdown-menu';
import { Card, CardContent, CardFooter, CardHeader } from './ui/card';
import { CommentForm } from './comments/CommentForm';
import {
  useNoteStats,
  EngagementPopover,
  filterSelf,
} from './EngagementStats';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog';

interface FeedItemProps {
  event: NostrEvent;
  showActions?: boolean;
}

export function FeedItem({ event, showActions = true }: FeedItemProps) {
  const { user } = useCurrentUser();
  const { mutate: publishEvent } = useNostrPublish();
  const { config } = useAppContext();
  const { defaultRelayUrl } = useDefaultRelay();
  const createRepost = useCreateRepost();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const author = useAuthor(event.pubkey);
  const stats = useNoteStats(event.id);
  const [showReplyForm, setShowReplyForm] = useState(false);
  const [showRawEvent, setShowRawEvent] = useState(false);

  const metadata = author.data?.metadata;
  const displayName = metadata?.name || metadata?.display_name || genUserName(event.pubkey);
  const npub = useMemo(() => nip19.npubEncode(event.pubkey), [event.pubkey]);
  const timeAgo = formatDistanceToNow(new Date(event.created_at * 1000), { addSuffix: true });

  const gateway = config.siteConfig?.nip19Gateway || 'https://nostr.at';
  const cleanGateway = gateway.endsWith('/') ? gateway.slice(0, -1) : gateway;
  const npubUrl = `${cleanGateway}/${npub}`;

  // Filter self from engagement lists
  const myPubkey = user?.pubkey;
  const reactionUsers = useMemo(() => filterSelf(stats.reactionUsers, myPubkey), [stats.reactionUsers, myPubkey]);
  const zapUsers = useMemo(() => filterSelf(stats.zapUsers, myPubkey), [stats.zapUsers, myPubkey]);
  const repostUsers = useMemo(() => filterSelf(stats.repostUsers, myPubkey), [stats.repostUsers, myPubkey]);
  const replyUsers = useMemo(() => filterSelf(stats.replyUsers, myPubkey), [stats.replyUsers, myPubkey]);

  const handleReact = () => {
    if (!user) {
      toast({
        title: "Authentication Required",
        description: "Please log in to react to posts.",
        variant: "destructive"
      });
      return;
    }

    publishEvent({
      event: {
        kind: 7,
        content: "+",
        tags: [
          ["e", event.id],
          ["p", event.pubkey]
        ]
      }
    }, {
      onSuccess: () => {
        toast({ title: "Liked", description: "Your reaction has been published." });
        queryClient.invalidateQueries({ queryKey: ['note-stats', event.id] });
      }
    });
  };

  const handleRepost = () => {
    if (!user) {
      toast({
        title: "Authentication Required",
        description: "Please log in to repost.",
        variant: "destructive"
      });
      return;
    }

    if (user.pubkey === event.pubkey) {
      toast({
        title: "Cannot repost",
        description: "You can't repost your own note from the feed.",
        variant: "destructive"
      });
      return;
    }

    createRepost.mutate({
      target: {
        id: event.id,
        pubkey: event.pubkey,
        kind: event.kind,
        content: event.content,
        tags: event.tags,
        created_at: event.created_at,
      },
      relayUrl: defaultRelayUrl || '',
      publishRelays: config.siteConfig?.publishRelays || [],
    }, {
      onSuccess: () => {
        toast({ title: "Reposted", description: "The note has been reposted." });
        queryClient.invalidateQueries({ queryKey: ['note-stats', event.id] });
      },
      onError: (error) => {
        toast({
          title: "Repost Failed",
          description: error.message,
          variant: "destructive"
        });
      }
    });
  };

  const handleBookmark = () => {
    if (!user) {
      toast({
        title: "Authentication Required",
        description: "Please log in to bookmark posts.",
        variant: "destructive"
      });
      return;
    }

    publishEvent({
      event: {
        kind: 10006,
        content: "",
        tags: [
          ["e", event.id]
        ]
      }
    }, {
      onSuccess: () => {
        toast({ title: "Bookmarked", description: "Post added to your bookmarks." });
      }
    });
  };

  const handleShare = () => {
    const noteId = nip19.noteEncode(event.id);
    const url = `${cleanGateway}/${noteId}`;
    navigator.clipboard.writeText(url);
    toast({ title: "Link Copied", description: "Post link copied to clipboard." });
  };

  const handleCopyEventId = () => {
    navigator.clipboard.writeText(event.id);
    toast({ title: "Event ID Copied", description: "Event ID copied to clipboard." });
  };

  // Engagement badge config — same color scheme as admin Notes section
  const engagementBadges = [
    {
      icon: Heart,
      count: stats.reactions,
      users: reactionUsers,
      title: 'Reactions',
      active: 'bg-red-500/10 border-red-500/20',
      iconActive: 'text-red-500 fill-red-500',
      textActive: 'text-red-500',
      label: String(stats.reactions),
      onClick: handleReact,
      disabled: false,
    },
    {
      icon: Zap,
      count: stats.zaps,
      users: zapUsers,
      title: `${stats.zapAmount} sats`,
      active: 'bg-yellow-500/10 border-yellow-500/20',
      iconActive: 'text-yellow-500 fill-yellow-500',
      textActive: 'text-yellow-500',
      label: stats.zapAmount > 0 ? `${stats.zaps} · ${stats.zapAmount.toLocaleString()}` : String(stats.zaps),
      onClick: null, // zap handled by ZapDialog wrapper
      disabled: false,
    },
    {
      icon: Repeat2,
      count: stats.reposts,
      users: repostUsers,
      title: 'Reposts',
      active: 'bg-green-500/10 border-green-500/20',
      iconActive: 'text-green-500',
      textActive: 'text-green-500',
      label: String(stats.reposts),
      onClick: handleRepost,
      disabled: createRepost.isPending,
    },
    {
      icon: MessageCircle,
      count: stats.replies,
      users: replyUsers,
      title: 'Replies',
      active: 'bg-blue-500/10 border-blue-500/20',
      iconActive: 'text-blue-500',
      textActive: 'text-blue-500',
      label: String(stats.replies),
      onClick: () => setShowReplyForm(!showReplyForm),
      disabled: false,
    },
  ];

  return (
    <Card className="overflow-hidden border-none sm:border shadow-none sm:shadow-sm bg-card mb-4">
      <CardHeader className="p-4 flex flex-row items-start justify-between space-y-0">
        <div className="flex items-center space-x-3">
          <a href={npubUrl} target="_blank" rel="noopener noreferrer">
            <Avatar className="h-10 w-10 border">
              <AvatarImage src={metadata?.picture} alt={displayName} />
              <AvatarFallback>{displayName.charAt(0).toUpperCase()}</AvatarFallback>
            </Avatar>
          </a>
          <div className="flex flex-col">
            <a href={npubUrl} target="_blank" rel="noopener noreferrer" className="font-bold hover:underline line-clamp-1">
              {displayName}
            </a>
            <div className="flex items-center text-xs text-muted-foreground space-x-1">
              <span>{timeAgo}</span>
            </div>
          </div>
        </div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={handleShare}>
              <Share2 className="mr-2 h-4 w-4" />
              Copy Link
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleCopyEventId}>
              <Copy className="mr-2 h-4 w-4" />
              Copy Event ID
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setShowRawEvent(true)}>
              <Code className="mr-2 h-4 w-4" />
              View raw event
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleBookmark}>
              <Bookmark className="mr-2 h-4 w-4" />
              Bookmark
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </CardHeader>

      <Dialog open={showRawEvent} onOpenChange={setShowRawEvent}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Raw event</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                navigator.clipboard.writeText(JSON.stringify(event, null, 2));
                toast({ title: "Copied", description: "Raw event JSON copied to clipboard." });
              }}
            >
              Copy JSON
            </Button>
            <pre className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 text-xs">
              {JSON.stringify(event, null, 2)}
            </pre>
          </div>
        </DialogContent>
      </Dialog>

      <CardContent className="px-4 pb-4 pt-0">
        <NoteContent event={event} className="text-base" centerMedia />
      </CardContent>

      {showActions && (
        <CardFooter className="px-2 py-1 border-t flex items-center justify-center gap-2">
          {engagementBadges.map((badge, i) => {
            const { icon: Icon, count, users, title, active, iconActive, textActive, label, onClick, disabled } = badge;
            const isActive = count > 0;
            const isZap = badge.icon === Zap;

            // The inner badge content — icon + count
            const badgeInner = (
              <div
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full border transition-all ${isActive
                  ? `${active} opacity-100`
                  : 'bg-muted/10 border-transparent opacity-40 hover:opacity-70'
                  } ${disabled ? 'opacity-50 pointer-events-none' : 'cursor-pointer'}`}
                title={title}
              >
                {disabled && <Loader2 className="h-4 w-4 animate-spin" />}
                {!disabled && <Icon className={`h-4 w-4 ${isActive ? iconActive : 'text-muted-foreground'}`} />}
                <span className={`text-xs font-semibold ${isActive ? textActive : 'text-muted-foreground'}`}>
                  {label}
                </span>
              </div>
            );

            // Zap badge: wrap in ZapDialog for the payment flow
            const interactiveBadge = isZap ? (
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              <ZapDialog target={event as any}>
                {badgeInner}
              </ZapDialog>
            ) : (
              <div onClick={disabled || !onClick ? undefined : onClick}>
                {badgeInner}
              </div>
            );

            // Wrap in engagement popover (hover/tap to see who interacted)
            // For zap, the popover wraps the dialog trigger so both work
            return (
              <EngagementPopover key={i} users={users} count={users.length}>
                {interactiveBadge}
              </EngagementPopover>
            );
          })}
        </CardFooter>
      )}

      {showReplyForm && (
        <div className="p-4 bg-muted/30 border-t">
          <CommentForm
            root={event}
            onSuccess={() => {
              setShowReplyForm(false);
              toast({ title: "Reply Published", description: "Your reply has been sent." });
              queryClient.invalidateQueries({ queryKey: ['note-stats', event.id] });
            }}
            compact
            placeholder="Write your reply..."
          />
        </div>
      )}
    </Card>
  );
}
