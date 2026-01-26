import { useState, useEffect } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { AlertCircle, Save, Rss, Plus, Trash2, UserPlus } from 'lucide-react';
import { useAppContext } from '@/hooks/useAppContext';
import { useNostrPublish } from '@/hooks/useNostrPublish';
import { useQueryClient } from '@tanstack/react-query';
import { useToast } from '@/hooks/useToast';
import { useCurrentUser } from '@/hooks/useCurrentUser';
import { useRemoteNostrJson } from '@/hooks/useRemoteNostrJson';
import { formatPubkey } from '@/lib/utils';

export default function AdminFeed() {
  const { config, updateConfig } = useAppContext();
  const { mutateAsync: publishEvent } = useNostrPublish();
  const queryClient = useQueryClient();
  const { user } = useCurrentUser();
  const { toast } = useToast();

  const [isSaving, setIsSaving] = useState(false);
  const [feedNpubs, setFeedNpubs] = useState<string[]>(config.siteConfig?.feedNpubs || []);
  const [readFromPublishRelays, setReadFromPublishRelays] = useState(config.siteConfig?.feedReadFromPublishRelays || false);
  const [newNpub, setNewNpub] = useState('');
  const { data: remoteNostrJson } = useRemoteNostrJson();

  useEffect(() => {
    if (config.siteConfig) {
      setFeedNpubs(config.siteConfig.feedNpubs || []);
      setReadFromPublishRelays(config.siteConfig.feedReadFromPublishRelays || false);
    }
  }, [config.siteConfig]);

  const handleSave = async () => {
    setIsSaving(true);
    try {
      const currentConfig = config.siteConfig || {};
      const updatedConfig = {
        ...currentConfig,
        feedNpubs,
        feedReadFromPublishRelays: readFromPublishRelays,
        updatedAt: Math.floor(Date.now() / 1000),
      };

      // We need to publish the entire site config because it's a replaceable event
      const configTags = [
        ['d', 'nostr-meetup-site-config'],
        ['title', updatedConfig.title || ''],
        ['logo', updatedConfig.logo || ''],
        ['favicon', updatedConfig.favicon || ''],
        ['og_image', updatedConfig.ogImage || ''],
        ['hero_title', updatedConfig.heroTitle || ''],
        ['hero_subtitle', updatedConfig.heroSubtitle || ''],
        ['hero_background', updatedConfig.heroBackground || ''],
        ['show_events', (updatedConfig.showEvents ?? true).toString()],
        ['show_blog', (updatedConfig.showBlog ?? true).toString()],
        ['max_events', (updatedConfig.maxEvents ?? 6).toString()],
        ['max_blog_posts', (updatedConfig.maxBlogPosts ?? 3).toString()],
        ['default_relay', updatedConfig.defaultRelay || ''],
        ['publish_relays', JSON.stringify(updatedConfig.publishRelays || [])],
        ['admin_roles', JSON.stringify(updatedConfig.adminRoles || {})],
        ['feed_npubs', JSON.stringify(feedNpubs)],
        ['feed_read_from_publish_relays', readFromPublishRelays.toString()],
        ['tweakcn_theme_url', updatedConfig.tweakcnThemeUrl || ''],
        ['section_order', JSON.stringify(updatedConfig.sectionOrder || [])],
        ['updated_at', updatedConfig.updatedAt.toString()],
      ];

      await publishEvent({
        event: {
          kind: 30078,
          content: JSON.stringify({ navigation: config.navigation }),
          tags: configTags,
        }
      });

      updateConfig((prev) => ({
        ...prev,
        siteConfig: updatedConfig,
      }));

      queryClient.clear();
      toast({
        title: "Settings Saved",
        description: "Feed configuration has been updated successfully.",
      });
    } catch (error) {
      console.error('Failed to save feed settings:', error);
      toast({
        title: "Error",
        description: "Failed to save feed settings. Please try again.",
        variant: "destructive",
      });
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Feed Management</h2>
          <p className="text-muted-foreground">
            Configure which Nostr users appear in your community feed.
          </p>
        </div>
        <Button onClick={handleSave} disabled={isSaving || !user}>
          <Save className="h-4 w-4 mr-2" />
          {isSaving ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Rss className="h-5 w-5 text-primary" />
            Feed Sources
          </CardTitle>
          <CardDescription>
            Configure the sources for your community feed.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Add from Directory</Label>
              <CardDescription>
                Select users from your community directory to add to the feed.
              </CardDescription>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {remoteNostrJson?.names && Object.entries(remoteNostrJson.names).map(([name, pubkey]) => {
                  const isAdded = feedNpubs.includes(pubkey);
                  return (
                    <Button
                      key={pubkey}
                      variant="outline"
                      size="sm"
                      className="justify-start gap-2 h-auto py-2"
                      disabled={isAdded}
                      onClick={() => {
                        if (!isAdded) {
                          setFeedNpubs(prev => [...prev, pubkey]);
                        }
                      }}
                    >
                      <Plus className="h-4 w-4" />
                      <div className="flex flex-col items-start overflow-hidden text-left">
                        <span className="font-medium truncate w-full">{name}</span>
                        <span className="text-[10px] text-muted-foreground truncate w-full">{pubkey}</span>
                      </div>
                    </Button>
                  );
                })}
              </div>
            </div>

            <Separator />

            <div className="space-y-2">
              <Label htmlFor="manualNpub">Add Manual npub</Label>
              <CardDescription>
                Add a specific Nostr public key (npub) to the feed sources.
              </CardDescription>
              <div className="flex gap-2">
                <Input
                  id="manualNpub"
                  value={newNpub}
                  onChange={(e) => setNewNpub(e.target.value)}
                  placeholder="npub1..."
                  className="flex-1"
                />
                <Button 
                  type="button"
                  onClick={() => {
                    if (newNpub.trim()) {
                      setFeedNpubs(prev => [...new Set([...prev, newNpub.trim()])]);
                      setNewNpub('');
                    }
                  }}
                >
                  <UserPlus className="h-4 w-4 mr-2" />
                  Add
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <Label>Current Feed Sources</Label>
              <div className="border rounded-md divide-y max-h-[300px] overflow-y-auto">
                {feedNpubs.length === 0 ? (
                  <div className="p-4 text-center text-sm text-muted-foreground">
                    No feed sources added yet.
                  </div>
                ) : (
                  feedNpubs.map((npub) => (
                    <div key={npub} className="flex items-center justify-between p-3 bg-card/50">
                      <div className="flex flex-col overflow-hidden mr-2">
                        <span className="text-sm font-mono truncate">{formatPubkey(npub)}</span>
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setFeedNpubs(prev => prev.filter(n => n !== npub));
                        }}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>

          <Separator />

          <div className="flex items-center justify-between p-4 border rounded-lg bg-muted/30">
            <div className="space-y-0.5">
              <Label className="text-base">Read from Publishing Relays</Label>
              <CardDescription>
                Also fetch notes from the publishing relays defined in Site Settings.
              </CardDescription>
            </div>
            <Switch
              checked={readFromPublishRelays}
              onCheckedChange={setReadFromPublishRelays}
            />
          </div>

          <div className="flex items-start gap-3 p-4 border rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-800 dark:text-blue-200 text-sm">
            <AlertCircle className="h-5 w-5 shrink-0" />
            <div>
              <p className="font-semibold">Note on Interactivity</p>
              <p className="mt-1">
                Any Nostr user visiting the public /feed page will be able to react, reply, zap, or bookmark these notes if they are logged in.
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
