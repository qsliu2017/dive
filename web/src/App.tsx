import { useEffect, useState } from "react";

export default function App() {
  const [layerIdList, setLayerIdList] = useState<string[]>([]);
  useEffect(() => {
    let isMounted = true;
    const doFetch = async () => {
      const res = await fetch("/api/layer");
      const data = await res.json().catch(console.error);
      console.log({ data });
      if (isMounted) setLayerIdList(data as string[]);
    };
    doFetch();
    return () => {
      isMounted = false;
    };
  }, []);
  if (!layerIdList) return <></>;
  return (
    <div>
      {layerIdList.map((id) => (
        <Layer id={id} key={id} />
      ))}
    </div>
  );
}

function Layer({ id }: { id: string }) {
  const [layer, setLayer] = useState<{
    id: string;
    index: number;
    command: string;
    size: number;
    treeId: string;
    names: string[];
    digest: string;
  } | null>(null);
  useEffect(() => {
    let isMounted = true;
    fetch(`/api/layer/${id}`)
      .then((res) => res.json())
      .then((data) => {
        if (isMounted) setLayer(data);
      });
    return () => {
      isMounted = false;
    };
  }, [id]);
  if (!layer) return <></>;
  return (
    <div>
      <h1>{layer.id}</h1>
      <p>{layer.command}</p>
      <p>{layer.size}</p>
      <p>{layer.treeId}</p>
      {layer.names && <p>{layer.names.join(", ")}</p>}
      <p>{layer.digest}</p>
    </div>
  );
}
