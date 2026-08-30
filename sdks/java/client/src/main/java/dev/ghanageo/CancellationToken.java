package dev.ghanageo;

import java.util.concurrent.CancellationException;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicBoolean;

/** Cooperative request cancellation shared by blocking, async, retry and download APIs. */
public final class CancellationToken {
  private static final CancellationToken NONE=new CancellationToken(false);
  private final AtomicBoolean cancelled=new AtomicBoolean();
  private final CopyOnWriteArrayList<Runnable> callbacks=new CopyOnWriteArrayList<>();
  private final boolean cancellable;
  private CancellationToken(boolean cancellable){this.cancellable=cancellable;}
  public static CancellationToken none(){return NONE;}
  public static Source source(){return new Source();}
  public boolean isCancelled(){return cancelled.get();}
  public void throwIfCancelled(){if(isCancelled())throw new CancellationException("GhanaGeo request cancelled");}
  AutoCloseable onCancel(Runnable callback){if(!cancellable)return()->{};if(cancelled.get()){callback.run();return()->{};}callbacks.add(callback);if(cancelled.get()&&callbacks.remove(callback))callback.run();return()->callbacks.remove(callback);}
  private void cancel(){if(cancellable&&cancelled.compareAndSet(false,true)){callbacks.forEach(Runnable::run);callbacks.clear();}}
  public static final class Source implements AutoCloseable {private final CancellationToken token=new CancellationToken(true);public CancellationToken token(){return token;}public void cancel(){token.cancel();}public void close(){cancel();}}
}
