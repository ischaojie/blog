import { useEffect, useState } from "react";

export default function Like({ source }: { source: string }) {
  const [count, setCount] = useState(0);
  const [like, SetLike] = useState(false);
  const ILIKEIT_URL = "https://ilikeit.chaojie.fun/";

  useEffect(() => {
    const getLikeIt = async () => {
      const data = await GetLikeCount(source);
      setCount(data.like_count);
    };
    getLikeIt();
  }, []);

  async function ILikeIt(source: string) {
    await fetch(`${ILIKEIT_URL}?source=${source}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
    }).catch((err) => console.error(err));
  }

  async function GetLikeCount(source: string) {
    return await fetch(`${ILIKEIT_URL}?source=${source}`, {
      method: "GET",
    }).then((res) => res.json());
  }

  function HandleClick(e: React.MouseEvent<HTMLAnchorElement, MouseEvent>) {
    e.preventDefault();

    ILikeIt(source);
    setCount(count + 1);
    SetLike(true);
  }

  return (
    <div>
      <a
        onClick={HandleClick}
        style={{
          padding: "0 0.4em",
          display: "inline-flex",
          alignItems: "center",
          backgroundColor: like ? "#EDEDED" : "#EFF7ED",
          color: like ? "#CCCCCC" : "#4F946E",
          border: "1px solid",
          borderColor: like ? "#EDEDED" : "#DAEDE4",
          borderRadius: "2px",
          cursor: "pointer",
        }}
      >
        {like ? <HeartFilled /> : <Heart />}
        <span style={{ padding: "0 4px" }}>{like ? "liked" : "like"}</span>
        <div style={{ padding: "0 4px" }}>{count}</div>
      </a>
    </div>
  );
}

function Heart() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="22"
      height="22"
      viewBox="0 0 24 24"
      strokeWidth="2"
      stroke="currentColor"
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path stroke="none" d="M0 0h24v24H0z" fill="none" />
      <path d="M19.5 12.572l-7.5 7.428l-7.5 -7.428a5 5 0 1 1 7.5 -6.566a5 5 0 1 1 7.5 6.572" />
    </svg>
  );
}

function HeartFilled() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="22"
      height="22"
      viewBox="0 0 24 24"
      strokeWidth="2"
      stroke="currentColor"
      fill="none"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path stroke="none" d="M0 0h24v24H0z" fill="none" />
      <path
        d="M6.979 3.074a6 6 0 0 1 4.988 1.425l.037 .033l.034 -.03a6 6 0 0 1 4.733 -1.44l.246 .036a6 6 0 0 1 3.364 10.008l-.18 .185l-.048 .041l-7.45 7.379a1 1 0 0 1 -1.313 .082l-.094 -.082l-7.493 -7.422a6 6 0 0 1 3.176 -10.215z"
        strokeWidth="0"
        fill="currentColor"
      />
    </svg>
  );
}
